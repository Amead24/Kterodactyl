/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gamev1alpha1 "github.com/kterodactyl/kterodactyl/api/v1alpha1"
)

const (
	// wsPingInterval is the interval for WebSocket ping/pong keepalive.
	// 30 seconds is compatible with Cloudflare Tunnel timeouts.
	wsPingInterval = 30 * time.Second

	// wsWriteWait is the deadline for writing a message to the WebSocket.
	wsWriteWait = 10 * time.Second

	// wsPongWait is the deadline for reading a pong message from the client.
	wsPongWait = 60 * time.Second

	// logTailLines is the number of historical log lines to stream on connect.
	logTailLines int64 = 100

	// logReadBufferSize is the buffer size for reading from the pod log stream.
	logReadBufferSize = 4096

	// writeChannelSize is the buffer size for the write channel.
	writeChannelSize = 256
)

// upgrader configures the WebSocket upgrade with buffer sizes.
// CheckOrigin returns true because the SPA is served via go:embed from the same origin.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// consoleMessage represents a message sent to/from the WebSocket client.
type consoleMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
}

// handleConsole is the WebSocket endpoint for streaming pod logs and executing commands.
// Authentication is via JWT token in the "token" query parameter (WebSocket connections
// cannot use Authorization headers during the upgrade handshake).
//
// GET /api/v1/gameservers/{name}/console?token=<jwt>
func (s *Server) handleConsole(w http.ResponseWriter, r *http.Request) {
	log := logf.FromContext(r.Context())

	// 1. Authenticate via query param
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token query parameter", http.StatusUnauthorized)
		return
	}

	claims, err := s.jwtService.ValidateToken(token)
	if err != nil {
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}

	namespace := claims.Namespace
	serverName := chi.URLParam(r, "name")

	// 2. Verify GameServer exists and belongs to user
	gs := &gamev1alpha1.GameServer{}
	if err := s.client.Get(r.Context(), client.ObjectKey{Name: serverName, Namespace: namespace}, gs); err != nil {
		http.Error(w, "game server not found", http.StatusNotFound)
		return
	}

	// 3. State check: only allow console when running
	if gs.Status.State != gamev1alpha1.GameServerStateReady &&
		gs.Status.State != gamev1alpha1.GameServerStateAllocated {
		http.Error(w, "server not running", http.StatusConflict)
		return
	}

	// 4. Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err, "failed to upgrade WebSocket connection")
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 5. Set up ping/pong keepalive
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	// Ping goroutine for Cloudflare Tunnel compatibility
	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 6. Write channel pattern (Pitfall 3: prevents concurrent write panics)
	writeCh := make(chan []byte, writeChannelSize)

	// Writer goroutine: single goroutine owns all writes to the WebSocket
	go func() {
		defer conn.Close()
		for {
			select {
			case msg, ok := <-writeCh:
				if !ok {
					// Channel closed; send close message and return
					conn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					log.Error(err, "failed to write WebSocket message")
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 7. Send initial connected message
	connectedMsg, _ := json.Marshal(consoleMessage{Type: "connected", Data: "streaming logs..."})
	select {
	case writeCh <- connectedMsg:
	default:
	}

	// 8. Start log streaming goroutine
	go s.streamPodLogs(ctx, cancel, namespace, serverName, writeCh, log)

	// 9. Command reading loop (main goroutine)
	s.readCommands(ctx, cancel, conn, namespace, serverName, writeCh, log)

	// 10. Cleanup
	cancel()
	close(writeCh)
}

// streamPodLogs streams pod logs via Follow=true and sends them through the write channel.
func (s *Server) streamPodLogs(ctx context.Context, cancel context.CancelFunc, namespace, serverName string, writeCh chan<- []byte, log interface {
	Error(error, string, ...interface{})
}) {
	tailLines := logTailLines
	req := s.clientset.CoreV1().Pods(namespace).GetLogs(serverName, &corev1.PodLogOptions{
		Container: "gameserver",
		Follow:    true,
		TailLines: &tailLines,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		errMsg, _ := json.Marshal(consoleMessage{Type: "error", Data: fmt.Sprintf("failed to stream logs: %v", err)})
		select {
		case writeCh <- errMsg:
		default:
		}
		return
	}
	defer stream.Close()

	buf := make([]byte, logReadBufferSize)
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			select {
			case writeCh <- data:
			case <-ctx.Done():
				return
			}
		}
		if err != nil {
			if err == io.EOF || ctx.Err() != nil {
				break
			}
			// Log stream interrupted
			break
		}
	}

	// Send stream ended message
	endMsg, _ := json.Marshal(consoleMessage{Type: "stream_ended"})
	select {
	case writeCh <- endMsg:
	default:
	}
}

// readCommands reads WebSocket messages from the client and executes commands in the pod.
func (s *Server) readCommands(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, namespace, serverName string, writeCh chan<- []byte, log interface {
	Error(error, string, ...interface{})
}) {
	defer cancel()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			// Client disconnected or error
			return
		}

		var msg consoleMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		if msg.Type != "command" || msg.Data == "" {
			continue
		}

		// Execute command in pod via remotecommand exec
		go s.execCommand(ctx, namespace, serverName, msg.Data, writeCh, log)
	}
}

// execCommand executes a command in the game server pod via SPDY exec.
func (s *Server) execCommand(ctx context.Context, namespace, serverName, command string, writeCh chan<- []byte, log interface {
	Error(error, string, ...interface{})
}) {
	req := s.clientset.CoreV1().RESTClient().Post().
		Resource("pods").Name(serverName).Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "gameserver",
			Command:   []string{"/bin/sh", "-c", fmt.Sprintf("echo '%s' > /proc/1/fd/0", command)},
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(s.restConfig, "POST", req.URL())
	if err != nil {
		errMsg, _ := json.Marshal(consoleMessage{Type: "error", Data: fmt.Sprintf("exec setup failed: %v", err)})
		select {
		case writeCh <- errMsg:
		default:
		}
		return
	}

	var stdout, stderr bytes.Buffer
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		errMsg, _ := json.Marshal(consoleMessage{Type: "error", Data: fmt.Sprintf("exec failed: %v", err)})
		select {
		case writeCh <- errMsg:
		default:
		}
		return
	}

	// Send stdout output if any
	if stdout.Len() > 0 {
		outMsg, _ := json.Marshal(consoleMessage{Type: "output", Data: stdout.String()})
		select {
		case writeCh <- outMsg:
		default:
		}
	}

	// Send stderr output if any
	if stderr.Len() > 0 {
		errMsg, _ := json.Marshal(consoleMessage{Type: "error", Data: stderr.String()})
		select {
		case writeCh <- errMsg:
		default:
		}
	}
}

// int64Ptr returns a pointer to the given int64 value.
func int64Ptr(i int64) *int64 {
	return &i
}

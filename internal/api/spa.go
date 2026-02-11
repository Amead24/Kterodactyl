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
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:frontend
var frontendFS embed.FS

// serveSPA returns an http.Handler that serves the embedded frontend SPA.
// For any path that does not correspond to an existing file in the embedded FS,
// it serves index.html to support client-side routing (SPA fallback).
func serveSPA() http.Handler {
	distFS, _ := fs.Sub(frontendFS, "frontend")
	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Try to open the file in the embedded FS
		f, err := distFS.Open(path)
		if err != nil {
			// File not found: serve index.html for client-side routing
			indexFile, _ := distFS.Open("index.html")
			defer indexFile.Close()
			stat, _ := indexFile.Stat()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile.(io.ReadSeeker))
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}

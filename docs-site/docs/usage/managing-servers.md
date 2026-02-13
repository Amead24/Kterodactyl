---
sidebar_position: 2
---

# Managing Servers

This guide covers the day-to-day operations for managing your game servers: viewing status, lifecycle actions, mod management, and the real-time console.

## Dashboard Overview

The main dashboard shows all your game servers with their current state, game type, and age. Each server card displays a color-coded status badge indicating its lifecycle state.

## Server Lifecycle States

Every GameServer progresses through a defined set of states:

| State | Description |
|-------|-------------|
| **Creating** | The operator is provisioning the Pod, Service, and networking resources. |
| **Starting** | The Pod is running but the game server process is still initializing. |
| **Ready** | The server is fully operational and accepting player connections. |
| **Allocated** | The server is marked as in-use (reserved for active gameplay sessions). |
| **Shutdown** | The server has been stopped. The GameServer resource still exists but no Pod is running. |
| **Error** | Something went wrong. Check the server detail page for condition messages describing the failure. |

### State Transitions

Not all state changes are valid. The operator enforces the following transition rules:

- **Creating** can move to Starting or Error
- **Starting** can move to Ready, Creating (Pod disappeared), Error, or Shutdown
- **Ready** can move to Allocated, Creating (restart), Error, or Shutdown
- **Allocated** can move to Ready, Creating (restart), Error, or Shutdown
- **Shutdown** can move to Creating (restart)
- **Error** can move to Shutdown or Creating (restart)

## Lifecycle Actions

From the server detail page, you can perform lifecycle actions using the buttons in the action bar:

### Start a Stopped Server

When a server is in the **Shutdown** state, click **Start** to restart it. The server transitions back to **Creating** and progresses through Starting to Ready. Your server configuration and parameters are preserved.

### Stop a Running Server

Click **Stop** on a running server (Ready or Allocated) to shut it down gracefully. The operator deletes the Pod and transitions the server to **Shutdown**. The GameServer resource and its configuration are preserved -- you can start it again later.

### Restart a Server

Click **Restart** on a running server to cycle it. The server transitions to **Creating**, which tears down the existing Pod and creates a fresh one. This is useful after uploading mods or when the server is in a bad state.

:::warning Data Persistence
Restarting a server replaces the Pod. World data persistence depends on the game's container image and volume configuration. Mods stored on the PVC are preserved across restarts.
:::

### Delete a Server

Click **Delete** to permanently remove a server. This action:

- Deletes the Pod (if running)
- Deletes the Service and HTTPRoute
- Deletes the mod storage PVC
- Removes the GameServer custom resource

**This action is irreversible.** All server data, mods, and configuration are lost. Create a backup first if you want to preserve your data.

## Mod Management

Kterodactyl supports uploading mod files to your game server through the web UI.

### Upload Mods

1. Navigate to the **Mods** tab on the server detail page
2. Drag and drop mod files onto the upload area, or click to browse your files
3. The files are uploaded to the server's mod storage PVC at the path defined by the game manifest (for example, `/mods` for Minecraft)

:::info Automatic Restart
After a successful mod upload, the server automatically restarts to ensure the mods are loaded. The server transitions to **Creating** and progresses back to **Ready** with the new mods active.
:::

### View Installed Mods

The Mods tab lists all files currently in the mod directory with their filenames and sizes.

### Remove Mods

Click the delete icon next to a mod file to remove it from the server. You may need to restart the server manually for the removal to take effect.

## Console

The console provides real-time access to your game server's log output and a command input for server administration.

### Viewing Logs

Navigate to the **Console** tab on the server detail page. Log output streams in real time via a WebSocket connection. You can see:

- Server startup messages
- Player join/leave events
- Error messages and warnings
- Plugin/mod output

### Sending Commands

Use the command input field at the bottom of the console to send commands directly to the game server process. For Minecraft, this includes commands like:

```
op PlayerName
difficulty hard
whitelist add PlayerName
say Server restarting in 5 minutes
```

The command is sent to the container's standard input via the WebSocket connection.

:::info Authentication
The console WebSocket connection uses your JWT token passed as a query parameter (since browser WebSocket connections cannot set HTTP headers). Authentication is handled automatically by the UI.
:::

## Server Metrics

The server detail page may show basic resource metrics (CPU and memory usage) if your cluster has the Kubernetes Metrics Server installed. If metrics are unavailable, a placeholder message is shown instead of an error.

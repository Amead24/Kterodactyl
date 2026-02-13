---
sidebar_position: 1
---

# Creating Servers

This guide walks through the complete workflow for creating a new game server in Kterodactyl.

## Log In

Open the Kterodactyl web UI in your browser. If this is your first visit, you will be redirected to the login page. Enter your username and password to authenticate.

:::tip First Time?
If you do not have an account yet, ask your cluster administrator for an invitation link. Registration requires a valid invite token.
:::

## Browse Available Games

After logging in, navigate to the **Games** page from the sidebar. This page displays all game types available on your cluster. Each game card shows the display name, container image, and default resource allocation.

The games listed here are loaded from the operator's built-in game definitions (the `games/` directory in the Kterodactyl image). Your administrator cannot add games through the UI -- new games are added by contributing game definitions to the project.

## Select a Game

Click on a game card (for example, **Minecraft Java Edition**) to open the server creation form. Kterodactyl dynamically generates this form from the game's JSON Schema definition, so every game has a tailored configuration experience.

## Configure Parameters

The configuration form presents fields specific to the selected game. For Minecraft, you will see fields like:

| Parameter | Description | Default |
|-----------|-------------|---------|
| **Accept EULA** | Must be `TRUE` to accept the [Minecraft EULA](https://www.minecraft.net/en-us/eula). This field is locked and cannot be changed. | `TRUE` |
| **Server Type** | The server implementation to use. Options include `VANILLA`, `PAPER`, `SPIGOT`, `FORGE`, `FABRIC`, and `QUILT`. | `VANILLA` |
| **Difficulty** | Game difficulty level: `peaceful`, `easy`, `normal`, or `hard`. | `normal` |
| **Game Mode** | Default mode for new players: `survival`, `creative`, `adventure`, or `spectator`. | `survival` |
| **Max Players** | Maximum number of concurrent players. Must be a positive integer. | `20` |
| **JVM Memory** | Java heap memory allocation: `1G`, `2G`, `4G`, or `8G`. | `2G` |
| **Server Message** | The message of the day displayed in the server browser (max 59 characters). | `A Kterodactyl Minecraft Server` |
| **PvP Enabled** | Whether player-vs-player combat is allowed. | `true` |
| **World Seed** | World generation seed. Leave empty for a random seed. | (empty) |
| **Online Mode** | Whether to require Mojang authentication for connecting players. | `true` |

Fields with dropdowns (like Server Type) are constrained by the game's schema -- you can only pick from the allowed values. Fields with validation patterns (like Max Players) will show an error if your input does not match.

## Create the Server

Once you are satisfied with the configuration, click **Create**. Kterodactyl will:

1. Validate your parameters against the game's JSON Schema
2. Create a `GameServer` custom resource in your personal namespace
3. Build a Pod with the game's container image and your parameters as environment variables

## Watch the Server Start

After creation, you are redirected to the server detail page. The server progresses through lifecycle states:

1. **Creating** -- The GameServer resource has been created and the operator is building the Pod, Service, and networking resources.
2. **Starting** -- The Pod exists and the container is running, but the game server process has not finished initializing yet.
3. **Ready** -- The game server is fully operational and accepting player connections.

The status badge on the server detail page updates in real time as the server progresses through these states.

## Connect to Your Server

Once the server reaches the **Ready** state, connection information appears on the server detail page:

- **Address**: The DNS name follows the pattern `game.username.baseDomain` (for example, `minecraft.alice.tonymead.org`). This requires your administrator to have configured a base domain and Gateway API routing.
- **Port**: The allocated port for the game protocol (for example, `25565` for Minecraft).

Copy the address and port into your game client to connect.

:::info DNS Not Configured?
If your cluster does not have a base domain configured, the address field will show the cluster-internal service name. In this case, use `kubectl port-forward` to access your server locally:

```bash
kubectl port-forward -n <your-namespace> svc/<server-name> 25565:25565
```
:::

## What Happens Behind the Scenes

When you create a server, the Kterodactyl operator:

1. Creates a **Pod** running the game container with your parameters as environment variables
2. Creates a **Service** exposing the game ports
3. Creates an **HTTPRoute** (Gateway API) for DNS-based routing (if base domain is configured)
4. Creates a **PVC** for mod storage (mounted at the game's mod path)
5. Sets **owner references** so all resources are cleaned up when the GameServer is deleted
6. Updates the GameServer **status** with connection information once the Pod is ready

# Tether
Tether streams your Discord presence and activities to your website via REST APIs and WebSocket connections, powered by a lightweight Go service that deploys in seconds. Built for developers, it automatically handles Discord IDs, hashes, and URLs. No manual translation needed.

Join the Tether Discord server [here](https://discord.gg/Ff5FfJzq) to start observing your presence. (discord.gg/Ff5FfJzq)

## Quickstart
Simplest fetch (browser JS):
```js
fetch("https://tether.eggwite.moe/v1/users/123456789012345678")
    .then((r) => r.json())
    .then((data) => console.log(data));
```


## What Tether is

- Tracks Discord gateway presences (with privileged intents enabled) and caches them in-memory.
- Serves a Lanyard-compatible REST shape with enriched fields (avatar_url, display/global names, primary_guild passthrough when present).
- Streams presence updates over WebSocket with simple opcodes (INIT_STATE, PRESENCE_UPDATE, heartbeats).
- Zero external deps by default (no Redis); optional admin slash commands for status/latency.

---

## API docs

### GET `/v1/users/{user_id}`

Retrieves the most recent cached presence data for the specified user. The response format mirrors Lanyard's API, incorporating its custom Spotify object with enriched URL fields for developer simplicity.

**API URL:** `https://tether.eggwite.moe/v1/users/{user_id}`

Example success response (200):
```json
{
  "active_clients": ["desktop"],
  "primary_active_client": "desktop",
  "listening_to_spotify": true,
  "spotify": {
    "track_id": "abcdef1234567890abcdef",
    "timestamps": { "start": 1234567890123, "end": 1234567890123 },
    "song": "Example Song",
    "artist": "Example Artist",
    "album_art_url": "https://example.com/album_art.webp",
    "album": "Example Album"
  },
  "discord_user": { /* user object (same as before) */ },
  "discord_status": "online",
  "activities": [ /* array of activity objects */ ]
}
```

Example error response (404 / not found):
```json
{
  "error": {
    "code": "user_not_monitored",
    "message": "User is not being monitored by Tether"
  }
}
```

Notes:
- Success responses return the presence object directly (no `success` envelope).
- Errors return an object under the `error` key with `code` and `message`.
- The low-level booleans `active_on_discord_*` are no longer exposed; use `active_clients` (list) and `primary_active_client` (string) instead.

### GET `/healthz`
Simple readiness probe. Returns 200 with `{ "status": "ok" }`.

## WebSocket docs

Connect to `wss://tether.eggwite.moe/socket` (compression optional via `?compression=zlib_json`).

Flow
- On connect you receive Opcode 1 (Hello) with `heartbeat_interval` in `d`. Send Opcode 3 (Heartbeat) on that cadence.
- Immediately after Hello, send Opcode 2 (Initialize) to choose what to watch.
- If you send an unknown opcode, the server closes the connection with 4004 `unknown_opcode`.
- Initialize must include a data object with at least one user ID; otherwise the server closes with 4005 `requires_data_object` or 4006 `invalid_payload`.

Initialize examples:
```js
{ "op": 2, "d": { "subscribe_to_ids": ["123", "456"] } }
{ "op": 2, "d": { "subscribe_to_id": "123" } }
```

Events (op 0):
- `INIT_STATE`: initial snapshot of requested presences.
- `PRESENCE_UPDATE`: incremental updates; includes `removed: true` when a user leaves/goes offline.

Example `INIT_STATE`:
```json
{
    "op": 0,
    "seq": 1,
    "t": "INIT_STATE",
    "d": {
        "123": { "user_id": "123", "data": { /* presence */ } }
    }
}
```

Example `PRESENCE_UPDATE`:
```json
{
    "op": 0,
    "seq": 2,
    "t": "PRESENCE_UPDATE",
    "d": { "user_id": "123", "data": { /* presence */ }, "removed": false }
}
```

Opcodes

| Opcode | Name       | Description                                           | Client |
| ------ | ---------- | ----------------------------------------------------- | ------ |
| 0      | Event      | Carries INIT_STATE and PRESENCE_UPDATE                | recv   |
| 1      | Hello      | Sent once on connect; includes `heartbeat_interval`   | recv   |
| 2      | Initialize | Client subscribes to specific user IDs               | send   |
| 3      | Heartbeat  | Client heartbeat at the interval from Hello          | send   |

Error codes

| Code | Meaning                  |
| ---- | ------------------------ |
| 4004 | unknown_opcode           |
| 4005 | requires_data_object     |
| 4006 | invalid_payload          |

Events (op 0):
- `INIT_STATE`: snapshot of requested presences.
- `PRESENCE_UPDATE`: incremental updates; includes `removed: true` when a user leaves or goes offline.

Example `INIT_STATE`:
```json
{
	"op": 0,
	"seq": 1,
	"t": "INIT_STATE",
	"d": {
		"94490510688792576": { "user_id": "94490510688792576", "data": { /* presence */ } }
	}
}
```

Example `PRESENCE_UPDATE`:
```json
{
	"op": 0,
	"seq": 2,
	"t": "PRESENCE_UPDATE",
	"d": { "user_id": "94490510688792576", "data": { /* presence */ } }
}
```

## Quick-start (JavaScript)

Browser example to connect, heartbeat, and subscribe to one user ID:

```js
const WS_URL = "wss://tether.eggwite.moe/socket";

let ws;
let heartbeat;

function connect() {
    ws = new WebSocket(WS_URL);

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        switch (msg.op) {
            case 1: { // Hello
                const interval = msg.d.heartbeat_interval;
                clearInterval(heartbeat);
                heartbeat = setInterval(() => ws?.send(JSON.stringify({ op: 3 })), interval);
                // Initialize subscription
                ws.send(JSON.stringify({ op: 2, d: { subscribe_to_id: "123456789012345678" } }));
                break;
            }
            case 0: {
                if (msg.t === "PRESENCE_UPDATE") {
                    console.log("presence", msg.d);
                }
                break;
            }
            default:
                // server will close with 4004 on unknown op
                break;
        }
    };

    ws.onclose = (ev) => {
        console.warn("socket closed", ev.code, ev.reason);
        clearInterval(heartbeat);
        setTimeout(connect, 1000); // simple retry
    };
}

connect();
```

Node.js: use the `ws` package and the same message flow.

## Self-host with Docker

Build from source (local Dockerfile)
```bash
git clone https://github.com/Eggwite/tether.git
cd tether

docker build -t tether:latest . # For multi-arch see below

cat > env.production <<'EOF'
DISCORD_TOKEN=your_token
GUILD_ID=your_guild_id
ADMIN_USER_IDS=comma_separated_ids
BEHIND_PROXY=false
PORT=8080
EOF

docker run -d \
    --name tether \
    -p 8080:8080 \
    --env-file env.production \
    tether:latest

curl http://localhost:8080/healthz # Health Check
```
Multi-arch build command

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t tether:latest .
```

Pull prebuilt image
```bash
cat > env.production <<'EOF'
DISCORD_TOKEN=your_token
GUILD_ID=your_guild_id
ADMIN_USER_IDS=comma_separated_ids
BEHIND_PROXY=false
PORT=8080
EOF

docker run -d \
    --name tether \
    -p 8080:8080 \
    --env-file env.production \
    eggwite/tether:latest

curl http://localhost:8080/healthz # Health Check
```

---

# Development

## Fast start (dev)

```bash
git clone https://github.com/Eggwite/tether.git
cd tether

cp .env.example .env  # edit DISCORD_TOKEN, GUILD_ID, ADMIN_USER_IDS

go run ./cmd

go test ./...
```

## Debian/Ubuntu quick setup

```bash
sudo apt-get update
sudo apt-get install -y golang

git clone https://github.com/Eggwite/tether.git
cd tether
cp .env.example .env  # edit values

go run ./cmd
```

## Configure via env (.env supported)

Required
- `DISCORD_TOKEN`: Discord bot token (for presence ingestion). If omitted, the service runs but has no Discord data.
- `GUILD_ID`: Guild ID to request members/presences for when privileged intents are enabled.

Recommended
- `ADMIN_USER_IDS`: Comma-separated Discord user IDs allowed to run slash commands (status/lag).

Optional
- `PORT`: HTTP listen port (default 8080)
- `APP_ENV`: production|prod|development|dev|debug
- `LOG_LEVEL`: debug|info|warn|error|fatal|panic (overrides APP_ENV log level)
- `BEHIND_PROXY`: true when behind a trusted proxy that sets X-Forwarded-For/CF-Connecting-IP

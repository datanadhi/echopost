# EchoPost

**EchoPost** is a short-lived helper process that kicks in when the main Data Nadhi server is down.
It stores incoming logs locally and safely pushes them to the server once things are back online.
After clearing the backlog, it shuts itself down.

---

## What it does
1. Listens on a local **UNIX socket** for logs from the SDK.
2. Saves all logs in a small **PebbleDB** store.
3. Keeps retrying delivery to the main server until it succeeds.
4. Logs all activity in JSON format for easy visibility.
5. Cleans up the database and socket before exiting.

---

## Why it exists
EchoPost is meant for **server environments** where temporary outages can happen —
for example, if the main pipeline or log server is restarting, or there’s a network cut.
Instead of losing data, logs are stored locally and sent once the system is healthy again.

It’s not a daemon or background service — it comes up, does its job, and exits cleanly.

---

## How it works (simple view)
```
App / SDK
   │
   ├──> sends logs to local EchoPost (via gRPC socket)
   │
EchoPost
   ├── stores logs in PebbleDB
   ├── retries delivery until success
   └── cleans up and exits
```

---

## Structure
```
echopost/
├── main.go                # Entry point
├── tools/                 # Core logic
│   ├── client.go          # Sends logs to server
│   ├── config.go          # File and path setup
│   ├── grpc_server.go     # gRPC socket and handlers
│   ├── log.go             # Simple JSON logger
│   └── pebble.go          # Queue and retry logic
├── logagent.proto         # gRPC proto definition
├── logagentpb/            # Generated gRPC code
│   ├── logagent.pb.go
│   └── logagent_grpc.pb.go
├── internal/
│   └── client/            # Local test client (excluded from builds)
│       └── main.go
├── go.mod
├── go.sum
└── .env                   # Optional local config
```

---

## For development
To build EchoPost locally:
```bash
go build -o echopost
```

To run with default options:
```bash
./echopost --datanadhi ./.datanadhi --health-url http://localhost:5000
```

To run the internal test client:
```bash
go run internal/client/main.go
```

---

## Notes
- The agent logs are stored under a `session-*` directory in the base folder.
- Once PebbleDB is empty and the server is healthy, EchoPost exits automatically.
- The internal client is excluded from packaging and releases.
- All logs printed by the agent are JSON formatted for easy parsing.

---

## Deployment

EchoPost has two automated deployment workflows configured via **GitHub Actions**.

| Type | Trigger | Behavior |
|------|----------|-----------|
| **Production Deploy** | On merge to `main` | Builds binaries for all supported platforms and pushes them to Cloudflare R2. Also updates the `latest` tag for easy download. |
| **Snapshot Deploy** | Manual trigger (via “Run workflow”) | Builds binaries for testing and pushes them with a suffix (e.g., `-snapshot`, `-rc1`, `-dev`). Useful for pre-release verification. |

Both workflows automatically pick up secrets from the configured environments in GitHub and deploy to the shared **downloads infrastructure** at:
```
https://downloads.datanadhi.com/echopost/<os>/<arch>/echopost-<version>
```

For example:
```
https://downloads.datanadhi.com/echopost/linux/amd64/echopost-v1.3.0
https://downloads.datanadhi.com/echopost/darwin/arm64/echopost-v1.3.0
https://downloads.datanadhi.com/echopost/linux/amd64/echopost-latest
```

### Snapshot Examples

When you trigger the snapshot workflow manually:
- If the base version in `VERSION` is `v1.3.0` and you specify `rc1` as the suffix,  
  it uploads as:  
  `echopost-v1.3.0-rc1`
- If you don’t specify anything, it defaults to:  
  `echopost-v1.3.0-snapshot`

These binaries appear under:
```
https://downloads.datanadhi.com/echopost/linux/amd64/echopost-v1.3.0-snapshot
```

### Security and Environments

- Both workflows use GitHub **environments** (`production` and `snapshot`)  
  to scope access to Cloudflare R2 credentials.
- Only maintainers with deployment permissions can trigger or approve releases.
- All deployments are atomic and immutable — once pushed, versioned binaries aren’t overwritten.

---

## License

Licensed under the [**GNU Affero General Public License v3.0 (AGPLv3)**](/LICENSE).


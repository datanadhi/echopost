# EchoPost

## Overview
EchoPost is a **short-lived agent** that exists only when the main server is unavailable.

Its only responsibility is to **avoid data loss** by temporarily buffering data locally and replaying it once the main server becomes available again.

EchoPost is intentionally minimal.  
It is not configurable, not long-running, and not meant to do anything beyond buffering and replay.

Once its job is completed, EchoPost shuts down.

![EchoPost Architecture](https://raw.githubusercontent.com/datanadhi/drawio/main/documentation/architecture/highlevel/export/echopost.svg)

---

## Lifecycle

### Startup
- EchoPost starts when the main server is detected as unavailable.
- A local gRPC server is started over a UNIX socket.
- Incoming data is accepted immediately.

### Buffering
- All incoming data is written to local storage in timestamp order.
- No data is deleted during this phase.

### Health checks
- EchoPost periodically checks if the main server is reachable.
- No replay happens until the server is confirmed healthy.

### Replay
- Buffered data is replayed in the same order it was received.
- Records are deleted **only after successful replay**.

### Shutdown
- When no buffered data remains, EchoPost shuts down gracefully.
- EchoPost does not remain idle after replay.

---

## Why is this required?
- When the main server goes down, there is a possibility of data loss.
- Avoiding this is the primary goal.
- Using an external fallback server increases latency and adds failure points.
- EchoPost runs on the same machine as the SDK/application, making communication faster and local.

---

## How is this different from agents out there today?
- Most agents are designed to send data to multiple destinations and require user configuration.  
  EchoPost does not do that. It **only talks to the Data Nadhi server**.
- Other agents are long-running and live until stopped manually or due to an exception.  
  EchoPost is **short-lived** and behaves more like a **process**.
- Long-running agents consume more memory because they do more work.  
  EchoPost is designed to **consume minimal memory** and only for a short duration.

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


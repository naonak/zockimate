# Zockimate

A Docker container manager with versioning capabilities, automated updates, and rollback features. Designed for **TrueNAS Scale** (24.10+), leveraging native ZFS integration for data snapshots.

## Features

- Automated container updates with safety rollback on failure
- ZFS snapshot integration for data versioning
- Scheduled updates and checks via cron expressions
- Snapshot history with search and filtering
- Apprise notifications (update available, success, failure)
- Multi-architecture support (amd64, arm64)

## Quick Start

### 1. Label your containers

```yaml
services:
  myapp:
    image: myapp:latest
    labels:
      - zockimate.enable=true
      - zockimate.zfs_dataset=ssd0/docker-apps/myapp  # optional
      - zockimate.timeout=5m                            # optional
```

### 2. Deploy zockimate

```yaml
services:
  zockimate:
    image: ghcr.io/naonak/zockimate:main
    cap_drop:
      - ALL
    cap_add:
      - SYS_ADMIN
      - SYS_MODULE
    devices:
      - /dev/zfs
    security_opt:
      - no-new-privileges
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - /proc:/proc
      - /etc/localtime:/etc/localtime:ro
      - ./data:/var/lib/zockimate:rw
    environment:
      - ZOCKIMATE_DB=/var/lib/zockimate/zockimate.db
      - ZOCKIMATE_LOG_LEVEL=info
      - ZOCKIMATE_APPRISE_URL=http://apprise:8000/notify  # optional
    command: schedule update "0 4 * * *"
```

This checks and updates all labeled containers daily at 4 AM with automatic rollback on failure.

## Usage

### One-shot commands

```bash
# Base docker run command (reuse for all examples below)
docker run --rm \
    --cap-drop=ALL \
    --cap-add=SYS_ADMIN \
    --cap-add=SYS_MODULE \
    --device=/dev/zfs \
    --security-opt=no-new-privileges \
    -v /mnt/data/zockimate:/var/lib/zockimate:rw \
    -v /proc:/proc \
    -v /etc/localtime:/etc/localtime:ro \
    -v /var/run/docker.sock:/var/run/docker.sock:ro \
    -e ZOCKIMATE_DB=/var/lib/zockimate/zockimate.db \
    ghcr.io/naonak/zockimate:main \
    [command]

# Check for updates
... check container1 container2

# Update containers
... update container1 container2

# Create snapshot
... save -m "Pre-migration" container1

# Rollback to latest snapshot
... rollback container1 --image --data --config

# Rollback to specific snapshot
... rollback container1 42 --image --data

# Show history
... history container1

# Remove old entries
... remove --older-than 30d container1

# Rename container
... rename old-name new-name
```

## Commands

### check [container...]

Checks for available updates without applying them.

```
Flags:
  -f, --force       Force check even with local image
  -c, --cleanup     Cleanup pulled images after check (default: true)
  -A, --all         Include stopped containers
  -N, --no-filter   Don't filter on zockimate.enable label
      --notify      Send notification via Apprise if updates are found
```

### update [container...]

Updates containers to their latest image versions. Creates a safety snapshot before updating, and automatically rolls back if the container fails to start.

```
Flags:
  -f, --force       Force update even if no new image available
  -n, --dry-run     Show what would be updated without making changes
  -A, --all         Include stopped containers
  -N, --no-filter   Don't filter on zockimate.enable label
      --notify      Send notification via Apprise on completion
```

### save [container...]

Creates snapshots of specified containers (config, image reference, ZFS data).

```
Flags:
  -m, --message     Message to attach to the snapshot
  -n, --dry-run     Show what would be saved without taking action
  -f, --force       Force snapshot even if container is stopped
      --no-cleanup  Skip cleanup of old snapshots
```

### rollback container [snapshot-id]

Restores a container to a previous state. Uses the latest snapshot if no ID is specified.

```
Flags:
  -i, --image     Rollback image
  -d, --data      Rollback data (ZFS snapshot)
  -c, --config    Rollback configuration
  -f, --force     Force rollback even if exact image version cannot be guaranteed
```

### history [container...]

Shows snapshot history for containers.

```
Flags:
  -n, --limit N     Limit number of entries
  -L, --last        Show only last entry per container
  -s, --sort-by     Sort by: date (default) or container
  -j, --json        Output in JSON format
  -q, --search      Search in messages and status
  -S, --since       Show entries since date (YYYY-MM-DD)
  -b, --before      Show entries before date (YYYY-MM-DD)
```

### remove [container...]

Removes snapshot entries from the database (and optionally ZFS snapshots and Docker containers).

```
Flags:
  -a, --all              Remove all entries for the container
      --older-than       Remove entries older than duration (e.g., 30d, 6m, 1y)
      --before           Remove entries before date (YYYY-MM-DD)
  -f, --force            Force removal
  -c, --with-container   Also stop and remove the Docker container
      --zfs              Also remove associated ZFS snapshots
  -n, --dry-run          Show what would be removed without taking action
```

### rename old-name new-name

Renames a container in Docker and updates all database references.

```
Flags:
  --db-only    Only rename in database, skip Docker rename
```

### schedule check|update "cron-expression" [container...]

Runs operations on a schedule. If no containers are specified, all labeled containers are processed.

```
Flags:
  -f, --force       Force operation even if no changes detected
  -n, --dry-run     Show what would happen without making changes
      --notify      Send notification via Apprise (default: true)
```

Cron expression format: `minute hour day-of-month month day-of-week`

Examples:
- `"0 4 * * *"` — daily at 4:00 AM
- `"0 */6 * * *"` — every 6 hours
- `"30 2 * * 1"` — Mondays at 2:30 AM

## Container Labels

| Label | Required | Description |
|-------|----------|-------------|
| `zockimate.enable` | Yes* | Set to `true` to include in management (bypassed with `--no-filter`) |
| `zockimate.zfs_dataset` | No | ZFS dataset path for data snapshots (e.g., `ssd0/docker-apps/myapp`) |
| `zockimate.timeout` | No | Per-container timeout as Go duration (e.g., `5m`, `30s`, max `24h`) |

\* Required unless using `--no-filter` / `-N` flag.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ZOCKIMATE_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `ZOCKIMATE_DB` | `zockimate.db` | Path to SQLite database file |
| `ZOCKIMATE_APPRISE_URL` | *(none)* | Apprise API URL for notifications |
| `ZOCKIMATE_RETENTION` | `10` | Number of snapshots to retain per container |
| `ZOCKIMATE_TIMEOUT` | `180` | Default operation timeout in seconds |

All environment variables can also be set via command-line flags (flags take precedence).

## Snapshot & Rollback System

### Snapshots

Each snapshot includes:
- Container configuration (config, host config, network config)
- Image reference (digest, tag, ID)
- ZFS dataset snapshot (if configured)
- Custom message for identification

Snapshots are created:
- Manually using `save` command
- Automatically before updates
- Automatically before rollbacks (safety snapshot)

Old snapshots are cleaned up according to the retention policy (default: 10 per container).

### Rollback Process

1. **Safety snapshot** — saves current state before any modification
2. **Image rollback** — restores the exact image version (digest preferred, falls back to tag/ID)
3. **Data rollback** — restores ZFS snapshot if configured
4. **Config rollback** — restores container configuration
5. **Verification** — waits for the container to become ready
6. **Failure recovery** — reverts to safety snapshot if any step fails after container modification

Use `--force` if the exact image version cannot be guaranteed (e.g., tag-only reference without digest).

## Troubleshooting

### ZFS "permission denied"

```
cannot create snapshots: permission denied
```

The container needs `SYS_ADMIN` capability for ZFS operations:
```yaml
cap_add:
  - SYS_ADMIN
  - SYS_MODULE
devices:
  - /dev/zfs
```

### SQLite "attempt to write a readonly database"

Check ownership of the database directory on the host. The container runs as root, so the mounted volume must be writable:
```bash
# On the host
chown root:root /path/to/zockimate/data/
chown root:root /path/to/zockimate/data/zockimate.db
chmod 755 /path/to/zockimate/data/
chmod 644 /path/to/zockimate/data/zockimate.db
```

On TrueNAS Scale, you may also need to reset NFSv4 ACLs on the directory.

### "failed to create ZFS snapshot: exit status 1"

Ensure the ZFS device is accessible and the `zockimate.zfs_dataset` label points to a valid dataset:
```bash
# Verify the dataset exists on the host
zfs list | grep myapp
```

## Building from Source

Requires Go 1.23+ and CGO (for SQLite):

```bash
CGO_ENABLED=1 go build -ldflags="-w -s" -o zockimate .
```

## License

MIT

# Zockimate
A Docker container manager with versioning capabilities, automated updates, and rollback features. This project is designed and optimized to work on TrueNAS Scale (ElectricEel-24.10.x), taking advantage of its native ZFS integration and container management capabilities.

## Features
- Automated container updates with safety rollback
- ZFS snapshot integration for data versioning
- Scheduled updates and checks
- Detailed update history
- Apprise notifications support

## Prerequisites
- **Docker**: Make sure Docker is installed and running on your system.
- **ZFS**: If using ZFS for dataset management, ensure ZFS is configured and operational.

## Usage

### One-shot commands
Use docker run for one-shot commands:
```bash
# Run a command
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
    -e ZOCKIMATE_LOG_LEVEL=info \
    ghcr.io/naonak/zockimate:main \
    [command]

# Examples
# Check for updates
docker run ... ghcr.io/naonak/zockimate:main check container1 container2

# Update containers
docker run ... ghcr.io/naonak/zockimate:main update container1 container2

# Create snapshot
docker run ... ghcr.io/naonak/zockimate:main save -m "Pre-update" container1

# Rollback
docker run ... ghcr.io/naonak/zockimate:main rollback container1 --image --data

# Show history
docker run ... ghcr.io/naonak/zockimate:main history container1

# Rename container
docker run ... ghcr.io/naonak/zockimate:main rename old-name new-name

# Rename only in database
docker run ... ghcr.io/naonak/zockimate:main rename --db-only old-name new-name
```

### Scheduled Updates
For scheduled updates, using docker-compose is recommended:

```yaml
services:
  zockimate-schedule:
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
      - ZOCKIMATE_APPRISE_URL=http://apprise:8000/notify
    # Update containers every day at 4 AM
    command: schedule update "0 4 * * *" container1 container2
    networks:
      - apprise_net
```

Enable a container for management by adding labels:
```yaml
services:
  myapp:
    labels:
      - zockimate.enable=true
      - zockimate.timeout=5m
      - zockimate.zfs_dataset=zroot/docker/myapp
```

## Commands

### check [container...]
Checks for container updates without applying them.
```
Options:
  -f, --force      Force check even with local image
  -c, --cleanup    Cleanup pulled images after check (default: true)
  -A, --all        Include stopped containers
  -N, --no-filter  Don't filter on zockimate.enable label
```

### update [container...]
Updates containers to their latest versions with automatic rollback on failure.
```
Options:
  -f, --force    Force update even if no new image available
  -n, --dry-run  Show what would be updated without making changes
  -A, --all      Include stopped containers
  -N, --no-filter Don't filter on zockimate.enable label
```

### save [container...]
Creates snapshots of specified containers.
```
Options:
  -m, --message    Message to attach to the snapshot
  -n, --dry-run    Show what would be saved without taking action
  -f, --force      Force snapshot even if container is stopped
  --no-cleanup     Skip cleanup of old snapshots
```

### rollback container [snapshot-id]
Restores container to a previous state. Uses latest snapshot if no ID specified.
```
Options:
  -i, --image    Rollback image
  -d, --data     Rollback data (ZFS snapshot)
  -c, --config   Rollback configuration
  -f, --force    Force rollback even if exact image version cannot be guaranteed
```

### history [container...]
Shows update history for containers.
```
Options:
  -n, --limit N    Limit number of entries per container
  -L, --last       Show only last entry per container
  -s, --sort-by    Sort by (date|container)
  -j, --json       Output in JSON format
  -q, --search     Search in messages and status
  -S, --since      Show entries since date (YYYY-MM-DD)
  -b, --before     Show entries before date (YYYY-MM-DD)
```

### rename old-name new-name
Renames a container in Docker and database.
```
Options:
  --db-only    Only rename in database, ignore Docker
```

### schedule check|update "cron-expression" [container...]
Schedules automatic operations.
```
Options:
  -f, --force    Force operation even if no changes needed
  -n, --dry-run  Show what would happen without making changes
```

## Snapshot & Rollback System

### Snapshots
Each snapshot includes:
- Container configuration
- Image reference (digest/tag/ID)
- ZFS dataset snapshot (if configured)
- Custom message for identification

Snapshots are created:
- Manually using `save` command
- Automatically before updates
- According to retention policy (default: 10)

### Rollback Process
1. **Pre-rollback**: Creates safety snapshot of current state
2. **Image Rollback**: Attempts to restore exact image version
   - Uses digest if available (guaranteed exact version)
   - Falls back to tag or ID
   - Requires `--force` if exact version cannot be guaranteed
3. **Data Rollback**: Restores ZFS snapshot if configured
4. **Config Rollback**: Restores container configuration
5. **Verification**: Ensures container starts properly
6. **Failure Handling**: Reverts to safety snapshot if any step fails

## License
MIT

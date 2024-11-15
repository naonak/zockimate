# Zockimate

A Docker container manager with versioning capabilities, automated updates, and rollback features.

## Features
- Automated container updates with safety rollback
- ZFS snapshot integration for data versioning
- Scheduled updates and checks
- Detailed update history
- Apprise notifications support

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

## License
MIT

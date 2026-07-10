# tsync

`tsync` keeps copies of individual files synchronized across local disks,
external drives, and USB devices on macOS and Linux.

- Changes made at any configured location are copied to the others.
- Missing drives are skipped and reconciled after they reappear.
- Writes use an atomic rename to avoid exposing partial files.
- launchd and systemd run tsync automatically in the background.

## Configuration

Create `~/.config/tsync/config.json` before installing:

```json
{
  "source_path": "/Users/j/Documents/hello.txt",
  "paths": [
    "/Users/j/Downloads/hello.txt",
    "/Volumes/USB/hello.txt"
  ],
  "conflict_policy": "newest_wins"
}
```

All paths must be absolute. Destination files may be absent, but their parent
directories must exist. tsync deliberately does not create missing parent
directories because doing so could hide the fact that a removable drive is
not mounted.

For more than one independent file:

```json
{
  "reconcile_interval": "2s",
  "syncs": [
    {
      "source_path": "/Users/j/Documents/hello.txt",
      "paths": ["/Volumes/USB/hello.txt"],
      "conflict_policy": "newest_wins"
    },
    {
      "source_path": "/Users/j/.ssh/config",
      "paths": ["/Volumes/Backup/ssh-config"],
      "conflict_policy": "source_wins"
    }
  ]
}
```

Policies:

- `newest_wins` (default): a changed file is propagated to every location;
  startup and remount conflicts use the newest modification time.
- `source_wins`: `source_path` is authoritative and overwrites changes made
  to its copies.

## Install

Release installation also registers and starts a per-user launchd or systemd
service. The configuration must exist and pass validation first.

```sh
curl -fsSL https://raw.githubusercontent.com/0jc1/tsync/main/install.sh -o install-tsync.sh
sh install-tsync.sh
rm install-tsync.sh
```

The default binary location is `~/.local/bin/tsync`. Use a different config or
install a system service with:

```sh
sh install-tsync.sh --config /path/to/config.json
sudo sh install-tsync.sh --scope system --config /etc/tsync/config.json
```

System installation places the binary in `/usr/local/bin`. Uninstallation
preserves configuration:

```sh
sh install-tsync.sh --uninstall
sudo sh install-tsync.sh --scope system --uninstall
```

## CLI

```text
tsync                         Run the sync process
tsync status                  Show runtime status
tsync validate                Validate the configuration
tsync help                    Show help
tsync version                 Show the version
tsync --config PATH validate  Use another configuration file
```

Running `tsync` without a command is how the startup service launches it.

## Native service controls

macOS user service:

```sh
launchctl kickstart -k "gui/$(id -u)/com.tsync.agent"
launchctl bootout "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.tsync.agent.plist"
```

Linux user service:

```sh
systemctl --user status tsync
systemctl --user restart tsync
systemctl --user stop tsync
```

After changing the configuration, restart the service.

## Build from source

```sh
go build -o tsync .
go test ./...
```

Building the binary directly does not register a startup service.

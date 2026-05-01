# docker-zeek

Docker deployment and management tool for [Zeek](https://zeek.org/)

[![Release](https://img.shields.io/github/v/release/activecm/docker-zeek)](https://github.com/activecm/docker-zeek/releases/latest)
[![CI](https://github.com/activecm/docker-zeek/actions/workflows/ci.yml/badge.svg)](https://github.com/activecm/docker-zeek/actions/workflows/ci.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/activecm/zeek)](https://hub.docker.com/r/activecm/zeek)
[![License](https://img.shields.io/github/license/activecm/docker-zeek)](LICENSE)

## What's Included

The Docker image comes with:

- Zeekctl cluster with AF_Packet
- Automatic process recovery if a worker crashes
- [ja3](https://github.com/salesforce/ja3) and [ja4+](https://github.com/FoxIO-LLC/ja4) TLS fingerprinting
- [zeek-open-connections](https://github.com/activecm/zeek-open-connections) for logging long-lived connections

## Quick Start

Requires [Docker](https://docs.docker.com/get-docker/) to be installed.

Download the CLI for your architecture from the [latest release](https://github.com/activecm/docker-zeek/releases/latest), then:

```bash
tar xzf zeek-linux-amd64.tar.gz
sudo mv zeek /usr/local/bin/zeek
sudo zeek start
```

On first run, the CLI prompts you to pick a network interface. Zeek logs are written to `/opt/zeek/logs/`.

> [!NOTE]
> On Rocky, RHEL, Alma, or CentOS, `/usr/local/bin` may not be in sudo's `secure_path`, which may cause `sudo zeek start` to fail. Use the full path (`sudo /usr/local/bin/zeek start`) or add `/usr/local/bin` to your sudo `secure_path`.

## Usage

### Commands

```
zeek start       Start the Zeek container
zeek stop        Stop the Zeek container
zeek restart     Restart the Zeek container
zeek status      Show container and process status
zeek readpcap    Process a pcap file offline
```

### Processing a Pcap

```bash
sudo zeek readpcap /path/to/capture.pcap [output-dir]
```

Logs default to `/opt/zeek/manual-logs/`.

### Sensor Setup

To re-run the interface selection:

```bash
sudo rm /opt/zeek/etc/node.cfg
sudo zeek start
```

### Adding Custom Packages

The image comes with `ja3`, `ja4`, and `zeek-open-connections`. There are two ways to add more.

For permanent additions, build a custom image that includes the package.

For trying a script-only package at runtime, you can install it directly in the running container with `zkg install`:

```bash
sudo docker exec zeek zkg install --skiptests <package>
sudo docker exec zeek zeekctl deploy
```

> [!NOTE]
> Runtime installs are ephemeral. They don't survive `zeek restart` or `zeek stop`. Use a custom image to keep packages permanently.

Compiled-plugin packages (those with C++ code) can't be installed at runtime because the final image doesn't include a compiler, so `zkg install` fails. Use a custom image instead.

#### Migrating from older versions

In v6 and earlier, the `zeek-zkg-script`, `zeek-zkg-plugin`, and `zeek-zkg-state` volumes held packages installed at runtime via `zkg`. v8 doesn't use those volumes.

The image already includes `ja3`, `ja4`, and `zeek-open-connections`. If you installed any other packages with `zkg install`, follow the custom image steps above to recreate them.

Then remove the old volumes:

```bash
sudo docker volume rm zeek-zkg-script zeek-zkg-plugin zeek-zkg-state
```

### Custom Zeek Scripts

Add custom scripts as `.zeek` files in `/opt/zeek/share/zeek/site/autoload/`. The container loads everything in that directory on each start. Use any filename other than `local.zeek`, which is regenerated automatically.

```bash
sudo cp custom.zeek /opt/zeek/share/zeek/site/autoload/210-custom.zeek
sudo zeek restart
```

## Logs

Zeek logs are written to `/opt/zeek/logs/` (or `$ZEEK_TOP_DIR/logs/` if customized).

## Configuration

### Host Directory

Zeek files live in `/opt/zeek/` by default. Change it with:

```bash
export ZEEK_TOP_DIR=/your/path
```

## Upgrading

Stop the running container, replace the CLI binary, and start again:

```bash
zeek stop
tar xzf zeek-linux-amd64.tar.gz
sudo mv zeek /usr/local/bin/zeek
sudo zeek start
```

Your `node.cfg` and `networks.cfg` are preserved. If you customized `zeekctl.cfg` or `100-default.zeek`, your previous version is saved as `.bak`. Reapply your changes to the new file.

If the CLI warns about orphaned zkg volumes from an older version: if you installed custom Zeek packages with `zkg install`, see [Migrating from older versions](#migrating-from-older-versions) first. Otherwise, remove them:

```bash
sudo docker volume rm zeek-zkg-script zeek-zkg-plugin zeek-zkg-state
```

After confirming the new container is working, free up disk space by removing the old image:

```bash
sudo docker images activecm/zeek      # show what's installed
sudo docker rmi activecm/zeek:6.2.1   # replace with the tag you had
```

## Development

```bash
make build              # build the CLI
make test               # run unit tests
make test-integration   # run integration tests
make lint               # run linter
make docker-build       # build the Docker image
make release            # build release artifacts
```

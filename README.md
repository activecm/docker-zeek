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

The image includes `ja3`, `ja4`, and `zeek-open-connections`, which other Active Countermeasures tools depend on. To try out additional packages, install them directly in the running container with `zkg`:

```bash
sudo docker exec zeek zkg install --skiptests <package>
sudo docker exec zeek zeekctl deploy
```

> [!NOTE]
> Runtime installs are ephemeral — they don't survive `zeek restart` or `zeek stop`. Compiled-plugin packages (those with C++ code) also can't be installed this way because the final image doesn't include a compiler.

To keep a package permanently, build your own image on top of `activecm/zeek`. Create a `Dockerfile`:

```dockerfile
FROM activecm/zeek:8.0.6

RUN zkg refresh && zkg install --force <package>
```

Build it:

```bash
sudo docker build -t my-zeek .
```

The `zeek` CLI always launches the upstream `activecm/zeek` image, so to use your custom build you'll need to run it directly with `docker` — see [Running without the CLI](#running-without-the-cli).

> [!NOTE]
> Packages with compiled plugins need build tools, which aren't in the base image. Install them (e.g. `RUN apk add --no-cache g++ make cmake bsd-compat-headers libpcap-dev openssl-dev zlib-dev`) before the `zkg install` step, plus any package-specific dependencies.

#### Migrating from older versions

In older versions of docker-zeek (v6 and prior), Zeek packages were managed using docker volumes. In v8, these volumes are unused.

To check whether you previously installed custom packages with `zkg install`, list the contents of the script volume:

```bash
sudo docker run --rm -v zeek-zkg-script:/check alpine ls /check
```

The v6 defaults are `bro-interface-setup`, `bro-doctor`, `ja3`, and `zeek-open-connections`. Anything else is a package you added.

- To keep using a custom package, bake it into your own image — see [Adding Custom Packages](#adding-custom-packages).
- Otherwise, remove the unused volumes:

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

## Running without the CLI

The container can run without the docker-zeek CLI. The examples below use the upstream `activecm/zeek:8.0.6` image; if you've built your own image (see [Adding Custom Packages](#adding-custom-packages)), substitute its name and tag wherever `activecm/zeek:8.0.6` appears.

### Minimum command to capture traffic

```bash
docker run -e ZEEK_INTERFACE=eth0 \
    --net=host --cap-add=NET_RAW --cap-add=NET_ADMIN \
    activecm/zeek:8.0.6
```

This brings up Zeek and captures host traffic. The container runs and produces logs inside its own filesystem. Logs are NOT persisted: when the container is removed, all logs are lost.

### Full setup with persistence

A ready-to-use example is at [`docker-compose.example.yml`](docker-compose.example.yml).

```yaml
services:
  zeek:
    image: activecm/zeek:8.0.6
    container_name: zeek
    restart: unless-stopped
    network_mode: host
    cap_add:
      - NET_RAW
      - NET_ADMIN
    environment:
      ZEEK_INTERFACE: eth0
    volumes:
      - /etc/localtime:/etc/localtime:ro
      - /opt/zeek/logs:/usr/local/zeek/logs
      - /opt/zeek/spool:/usr/local/zeek/spool
```

To start it:

```bash
sudo docker compose up -d
```

What the extra pieces do:
- `restart: unless-stopped` brings the container back if it crashes or the host reboots.
- The `/opt/zeek/logs` and `/opt/zeek/spool` mounts together persist logs to the host. Live log files are written to `/usr/local/zeek/spool/<node>/`, rotated `.log.gz` files are in `/usr/local/zeek/logs/<date>/`. Both mounts are needed to access both.
- The `/etc/localtime` mount makes log timestamps use the host timezone instead of UTC.

### Notes

Multiple interfaces are specified by comma-separating: `ZEEK_INTERFACE: eth0,eth1`. To override the auto-detected worker count, add `ZEEK_WORKERS: N`. To use your own custom node.cfg instead of the env-var path, replace the `ZEEK_INTERFACE` env var with a bind mount: `- /path/to/node.cfg:/usr/local/zeek/etc/node.cfg`.

Only one Zeek container can run on the host at a time because of `network_mode: host` and Zeek's Prometheus telemetry binding to ports 9991 and 9992. If our CLI is already running a Zeek container, stop it first with `zeek stop`.

## Upgrading

Stop the running container, replace the CLI binary, and start again:

```bash
zeek stop
tar xzf zeek-linux-amd64.tar.gz
sudo mv zeek /usr/local/bin/zeek
sudo zeek start
```

Your `node.cfg` and `networks.cfg` are preserved. If you customized `zeekctl.cfg` or `100-default.zeek`, your previous version is saved as `.bak`. Reapply your changes to the new file.

If the CLI warns about orphaned zkg volumes from an older version, see [Migrating from older versions](#migrating-from-older-versions).

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

# docker-zeek

Run a multi-process [Zeek](https://zeek.org/) cluster in Docker.

[![Release](https://img.shields.io/github/v/release/activecm/docker-zeek)](https://github.com/activecm/docker-zeek/releases/latest)
[![CI](https://github.com/activecm/docker-zeek/actions/workflows/ci.yml/badge.svg)](https://github.com/activecm/docker-zeek/actions/workflows/ci.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/activecm/zeek)](https://hub.docker.com/r/activecm/zeek)
[![License](https://img.shields.io/github/license/activecm/docker-zeek)](LICENSE)

## What's Included

The Docker image comes with:

- Multi-process zeekctl cluster with AF_Packet
- Automatic process recovery if a worker crashes
- [ja3](https://github.com/salesforce/ja3) and [ja4+](https://github.com/FoxIO-LLC/ja4) TLS fingerprinting
- [zeek-open-connections](https://github.com/activecm/zeek-open-connections) for logging long-lived connections

## Quick Start

Requires [Docker](https://docs.docker.com/get-docker/) to be installed.

Download the CLI for your architecture from the [latest release](https://github.com/activecm/docker-zeek/releases/latest), then:

```bash
tar xzf zeek-linux-amd64.tar.gz
sudo mv zeek /usr/local/bin/zeek
zeek start
```

On first run, the CLI will prompt you to pick a network interface and number of worker processes. Zeek logs are written to `/opt/zeek/logs/`.

## Usage

### Commands

```
zeek start       Start the Zeek container
zeek stop        Stop the Zeek container
zeek restart     Restart the Zeek container
zeek status      Show container and process status
zeek update      Pull the latest image and restart
zeek enable      Start Zeek on boot
zeek disable     Stop Zeek from starting on boot
zeek readpcap    Process a pcap file offline
```

### Processing a Pcap

```bash
zeek readpcap /path/to/capture.pcap [output-dir]
```

Logs default to `/opt/zeek/manual-logs/`.

### Sensor Setup

To re-run the interface selection wizard:

```bash
sudo rm /opt/zeek/etc/node.cfg
zeek start
```

### Installing Zeek Packages

```bash
docker exec -it zeek zkg install hassh
zeek restart
```

### Custom Zeek Scripts

Drop `.zeek` files into the autoload directory. They get included alphabetically to build `local.zeek` on container start. `local.zeek` is regenerated every time the container starts, so don't edit it directly.

```bash
sudo cp custom.zeek /opt/zeek/share/zeek/site/autoload/210-custom.zeek
zeek restart
```

## Logs

Zeek logs are written to `/opt/zeek/logs/` (or `$ZEEK_TOP_DIR/logs/` if customized). Logs are rotated hourly and organized into date-stamped directories.

## Configuration

### Host Directory

Zeek files live in `/opt/zeek/` by default. Change it with:

```bash
export ZEEK_TOP_DIR=/your/path
```

### Image Version

The CLI pulls the Docker image version it was built for. To use a different [published version](https://hub.docker.com/r/activecm/zeek/tags):

```bash
export ZEEK_RELEASE=8.0.6
```

## Development

```bash
make build              # build the CLI
make test               # run unit tests
make test-integration   # run integration tests (requires Docker)
make lint               # run linter
make docker-build       # build the Docker image
make release            # build release artifacts
```

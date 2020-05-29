#!/bin/bash

# Allow customization
ZEEKLOGS="${1:-$(pwd)/logs}"
NODECFG="${2:-$(pwd)/node.cfg}"

# Remove the old container
echo "Stopping previous Zeek instance..."
docker stop --time 90 zeek >/dev/null
docker rm zeek >/dev/null

if [ ! -d "$ZEEKLOGS" ]; then
  echo "Creating Zeek log directory..."
  mkdir -p "$ZEEKLOGS"
  [ ! -d "$ZEEKLOGS" ] && echo "Error: Could not create Zeek log directory!" && exit 1
fi

if [ ! -s "$NODECFG" ]; then
  echo "Creating node.cfg file..."
  mkdir -p "$(dirname "$NODECFG")"
  touch "$NODECFG"
  docker run --rm -it --network host \
    --mount source="$NODECFG",destination=/node.cfg,type=bind \
    activecm/zeek \
    zeekcfg -o /node.cfg --type afpacket --no-pin
  [ ! -s "$NODECFG" ] && echo "Error: Could not create node.cfg file!" && exit 1
fi

# Create and start a new one
echo "Starting Zeek..."
docker run --cap-add net_raw --cap-add net_admin --network host --detach \
    --name zeek \
    --restart always \
    --mount source=/etc/localtime,destination=/etc/localtime,type=bind,readonly \
    --mount source="$ZEEKLOGS",destination=/usr/local/zeek/logs/,type=bind \
    --mount source="$NODECFG",destination=/usr/local/zeek/etc/node.cfg,type=bind \
    activecm/zeek >/dev/null

# Display stats to show its running
docker ps --filter name=zeek
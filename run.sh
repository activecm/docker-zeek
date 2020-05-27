#!/bin/bash

# Remove the old container
echo "Stopping previous container..."
docker stop --time 90 zeek
docker rm zeek

# Create and start a new one
echo "Starting new container..."
docker run --cap-add net_raw --cap-add net_admin --network host --detach \
    --name zeek \
    --restart always \
    --mount source=/etc/localtime,destination=/etc/localtime,type=bind,readonly \
    --mount source=$(pwd)/logs,destination=/usr/local/zeek/logs/,type=bind \
    --mount source=$(pwd)/node.cfg,destination=/usr/local/zeek/etc/node.cfg,type=bind \
    activecm/zeek

# Display stats to show its running
docker ps --filter name=zeek
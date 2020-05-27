#!/bin/bash

ZEEK_VERSION="${1:-3.1.3}"
ZEEKCFG_VERSION="${2:-0.0.3}"

docker build --build-arg ZEEK_VERSION=$ZEEK_VERSION --build-arg ZEEKCFG_VERSION=$ZEEKCFG_VERSION -t activecm/zeek:$ZEEK_VERSION .
# Verify version number
docker run --rm activecm/zeek:$ZEEK_VERSION zeek --version

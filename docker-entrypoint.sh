#!/bin/bash

set -e

# run zeekctl diag on error
diag() {
	echo "Running zeekctl diag for debugging"
	zeekctl diag
	trap - ERR
}
trap 'diag' ERR

# do final log rotation
stopzz() {
	echo "Stopping zeek..."
	zeekctl stop
	trap - SIGKILL SIGINT SIGTERM
}
trap 'stop' SIGKILL SIGINT SIGTERM

# ensure Zeek has a valid, updated config, and then start Zeek
zeekctl check
zeekctl install
zeekctl start

# periodically run the Zeek cron monitor
zeekctl cron enable
crond -f -L /dev/fd/1

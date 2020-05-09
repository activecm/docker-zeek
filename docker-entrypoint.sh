#!/bin/bash

# exit script if an error is encountered
set -e

# do final log rotation
stop() {
	echo "Stopping zeek..."
	zeekctl stop
	trap - SIGINT SIGTERM
	exit
}

# run zeekctl diag on error
diag() {
	echo "Running zeekctl diag for debugging"
	zeekctl diag
	trap - ERR
}

trap 'diag' ERR

# ensure Zeek has a valid, updated config, and then start Zeek
zeekctl check
zeekctl install
zeekctl start

# ensure spool logs are rotated when container is stopped
trap 'stop' SIGINT SIGTERM

# periodically run the Zeek cron monitor to restart any terminated processes
zeekctl cron enable
# disable the zeekctl ERR trap as there are no more zeek commands to fail
trap - ERR

# daemonize cron but log output to stdout
crond -b -L /dev/fd/1

# infinite loop to prevent container from exiting and allow processing of signals
while :; do sleep 1s; done

#!/bin/bash
#Sample start/stop script for Zeek running inside docker
#based on service_script_template v0.2
#Many thanks to Logan for his Active-Flow init script, from which some of the following was copied.
#Many thanks to Ethan for his help with the design and implementation
#V0.4.0

#==== USER CUSTOMIZATION ====
#The default Zeek top level directory (/opt/zeek) can be overridden with
#the "zeek_top_dir" environment variable.  Edit /etc/profile.d/zeek and 
#add the line (without leading "#"):
#export zeek_top_dir='/my/data/zeek/'
#
#Similarly, the preferred release of zeek ("3.0", which covers any 3.0.x
#version) can be overridden with the "zeek_release" variable.  Edit the
#/etc/profile.d/zeek file and add the line (without leading "#"):
#export zeek_release='lts'
#
#You'll need to log out and log back in again for these lines to take effect.

if [ -n "$1" -a -z "$2" ]; then
	case "$1" in
	start|stop|restart|force-restart|status|reload|enable|disable|pull|update)
		action="$1"
		;;
	*)
		echo "Unrecognized action $1 , exiting" >&2
		exit 1
		;;
	esac
else
	echo 'This script expects a single command line option (start, stop, restart, status, reload, enable or disable).  Please run again.  Exiting' >&2
	exit 1
fi

#The user can set the top level directory that holds all zeek content by setting it in "zeek_top_dir" (default "/opt/zeek")
HOST_ZEEK=${zeek_top_dir:-/opt/zeek}
#Note, we force the 3.0 release for stability, though the user can override it by setting the "zeek_release" environment variable
IMAGE_NAME="activecm/zeek:${zeek_release:-3.0}"
CONTAINER_NAME="zeek"

# If the current user doesn't have docker permissions run with sudo
SUDO=''
if [ ! -w "/var/run/docker.sock" ]; then
	SUDO="sudo --preserve-env "
fi

if $SUDO docker inspect "$CONTAINER_NAME" &>/dev/null; then
	CONTAINER_RUNNING=`$SUDO docker inspect -f "{{ .State.Running }}" $CONTAINER_NAME 2>/dev/null`
	RESTART_POLICY=`$SUDO docker inspect -f "{{ .HostConfig.RestartPolicy.Name }}" $CONTAINER_NAME 2>/dev/null`
else
	CONTAINER_RUNNING="false"
	RESTART_POLICY="always"
fi


case "$action" in
start)
	#Command(s) needed to start the service right now

	if [ "$CONTAINER_RUNNING" == "true" ]; then
		echo "Zeek is already running." >&2
		exit 0
	fi

	# create bare minimum directories required for running Zeek
	if [ ! -d "$HOST_ZEEK/logs" ] || [ ! -d "$HOST_ZEEK/spool" ] || [ ! -d "$HOST_ZEEK/etc" ]; then
		sudo mkdir -p "$HOST_ZEEK/logs" "$HOST_ZEEK/spool" "$HOST_ZEEK/etc"
	fi

	# create the node.cfg file required for running Zeek
	if [ ! -s "$HOST_ZEEK/etc/node.cfg" ]; then
		echo "Could not find $HOST_ZEEK/etc/node.cfg. Generating one now." >&2

		$SUDO docker run --rm -it --network host \
		--mount source="$HOST_ZEEK/etc/",destination=/zeek/,type=bind \
		"$IMAGE_NAME" \
		zeekcfg -o /zeek/node.cfg --type afpacket
	fi

	# create the volumes required for peristing user-installed zkg packages
	$SUDO docker volume create zeek-zkg-script >/dev/null
	$SUDO docker volume create zeek-zkg-plugin >/dev/null
	$SUDO docker volume create zeek-zkg-state >/dev/null

	docker_cmd=("docker" "run" "--detach")  # start container in the background
	docker_cmd+=("--name" "$CONTAINER_NAME") # provide a predictable name
	docker_cmd+=("--restart" "$RESTART_POLICY")
	docker_cmd+=("--cap-add" "net_raw" "--cap-add" "net_admin") # allow Zeek to listen to raw packets
	docker_cmd+=("--network" "host") # allow Zeek to monitor host network interfaces
	# allow packages installed via zkg to persist across restarts
	docker_cmd+=("--mount" "source=zeek-zkg-script,destination=/usr/local/zeek/share/zeek/site/packages/,type=volume")
	docker_cmd+=("--mount" "source=zeek-zkg-plugin,destination=/usr/local/zeek/lib/zeek/plugins/packages/,type=volume")
	docker_cmd+=("--mount" "source=zeek-zkg-state,destination=/root/.zkg,type=volume")
	# mirror the host timezone settings to the container
	docker_cmd+=("--mount" "source=/etc/localtime,destination=/etc/localtime,type=bind,readonly")
	# persist and allow accessing the logs from the host
	docker_cmd+=("--mount" "source=$HOST_ZEEK/logs,destination=/usr/local/zeek/logs/,type=bind")
	docker_cmd+=("--mount" "source=$HOST_ZEEK/spool,destination=/usr/local/zeek/spool/,type=bind")
	# allow users to provide arbitrary custom config files and scripts
	# mount all zeekctl config files
	while IFS=  read -r -d $'\0' CONFIG; do
    	docker_cmd+=("--mount" "source=$CONFIG,destination=/usr/local/zeek/${CONFIG#"$HOST_ZEEK"},type=bind")
	done < <(find "$HOST_ZEEK/etc/" -type f -print0 2>/dev/null)
	# mount all zeek scripts
	while IFS=  read -r -d $'\0' SCRIPT; do
    	docker_cmd+=("--mount" "source=$SCRIPT,destination=/usr/local/zeek/${SCRIPT#"$HOST_ZEEK"},type=bind")
	done < <(find "$HOST_ZEEK/share/" -type f -print0 2>/dev/null)
		# loop reference: https://stackoverflow.com/a/23357277
		# ${CONFIG#"$HOST_ZEEK"} strips $HOST_ZEEK prefix
	docker_cmd+=("$IMAGE_NAME")

	echo "Starting the Zeek docker container" >&2
	$SUDO "${docker_cmd[@]}"

	# Fix current symlink for the host (sleep to give Zeek time to finish starting)
	(sleep 15s; $SUDO docker exec "$CONTAINER_NAME" ln -sfn "$HOST_ZEEK/spool/manager" /usr/local/zeek/logs/current) &

	;;
stop)
	#Command(s) needed to stop the service right now

	if [ "$CONTAINER_RUNNING" == "false" ]; then
		echo "Zeek is already stopped." >&2
		exit 0
	fi

	echo "Stopping the Zeek docker container" >&2
	$SUDO docker stop -t 90 "$CONTAINER_NAME" >&2

	echo "Removing the Zeek docker container" >&2
	$SUDO docker rm --force "$CONTAINER_NAME" >&2
	;;

restart|force-restart)
	#Command(s) needed to stop and start the service right now
	#You can test the value of "$action" in case there's a different set of steps needed to "force-restart"
	echo "Restarting the Zeek docker container" >&2
	$0 stop
	$0 start
	;;

status)
	#Command(s) needed to tell the user the state of the service
	echo "Zeek docker container status" >&2
	$SUDO docker ps --filter name=zeek >&2

	echo "Zeek processes status" >&2
	$SUDO docker exec "$CONTAINER_NAME" zeekctl status >&2
	;;

reload)
	#Command(s) needed to tell the service to reload any configuration files
	echo "Reloading Zeek docker container configuration files" >&2
	#Note; I'm not aware of a way to do a config file reload, so forcing a full restart at the moment.
	$0 stop
	$0 start
	;;

enable)
	#Command(s) needed to start the service on future boots
	echo "Enabling Zeek docker container on future boots" >&2
	if [ "$CONTAINER_RUNNING" == "false" ]; then
		echo "Zeek is stopped - please start first to set restart policy." >&2
		exit 0
	fi

	$SUDO docker update --restart always "$CONTAINER_NAME" >&2
	;;

disable)
	#Command(s) needed to stop the service on future boots
	echo "Blocking Zeek docker container from starting on future boots" >&2
	if [ "$CONTAINER_RUNNING" == "false" ]; then
		echo "Zeek is stopped - please start first to set restart policy." >&2
		exit 0
	fi

	$SUDO docker update --restart no "$CONTAINER_NAME" >&2
	;;

pull|update)
	#Command needed to pull down a new version of Zeek if there's a new docker image
	$SUDO docker pull "$IMAGE_NAME"

	$0 stop
	$0 start
	;;

*)
	echo "Unrecognized action $action , exiting" >&2
	exit 1
	;;
esac

exit 0

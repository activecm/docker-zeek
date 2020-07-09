#!/bin/bash
#Sample start/stop script for Zeek running inside docker
#based on service_script_template v0.2
#Many thanks to Logan for his Active-Flow init script, from which some of the following was copied.
#Many thanks to Ethan for his help with the design and implementation
#V0.3.5

#==== USER CUSTOMIZATION ====
#The default Zeek top level directory (/opt/zeek) can be overridden with
#the "zeek_top" environment variable.  Edit /etc/profile.d/zeek and 
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
host_zeek=${zeek_top_dir:-/opt/zeek}

host_zeek_logs="$host_zeek/logs"
host_zeek_spool="$host_zeek/spool"
host_zeek_etc="$host_zeek/etc"
host_zeek_node_cfg="$host_zeek_etc/node.cfg"

CONTAINER_NAME="zeek"
#Note, we force the 3.0 release for stability, though the user can override it by setting the "zeek_release" environment variable
host_zeek_release=${zeek_release:-3.0}
IMAGE_NAME="activecm/zeek:$host_zeek_release"

# If the current user doesn't have docker permissions run with sudo
SUDO=''
if [ ! -w "/var/run/docker.sock" ]; then
	SUDO="sudo --preserve-env "
fi

sudo --preserve-env mkdir -p "$host_zeek" "$host_zeek_logs" "$host_zeek_spool" "$host_zeek_etc"

#See if we need to download the image first.  Note, the 3.0 release is the default via the IMAGE_NAME variable.
if [ -z "`docker images $IMAGE_NAME | grep -v '^REPOSITORY'`" ]; then
	$SUDO docker pull "$IMAGE_NAME"
fi

if [ ! -s "$host_zeek_node_cfg" ]; then
	echo "We do not appear to have a node.cfg file, so generating one now." >&2

	sudo --preserve-env touch "$host_zeek_node_cfg"
	$SUDO docker run --rm -it --network host \
	 --mount source="$host_zeek_node_cfg",destination=/node.cfg,type=bind \
	 "$IMAGE_NAME" \
	 zeekcfg -o /node.cfg --type afpacket
fi

if $SUDO docker inspect "$CONTAINER_NAME" &>/dev/null; then
	CONTAINER_EXIST="true"
	CONTAINER_RUNNING=`$SUDO docker inspect -f "{{ .State.Running }}" $CONTAINER_NAME 2>/dev/null`
	RESTART_POLICY=`$SUDO docker inspect -f "{{ .HostConfig.RestartPolicy.Name }}" $CONTAINER_NAME 2>/dev/null`
else
	CONTAINER_EXIST="false"
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

	echo "Starting the Zeek docker container" >&2
	$SUDO docker run --cap-add net_raw --cap-add net_admin --network host --detach \
	 --name "$CONTAINER_NAME" \
	 --restart always \
	 --mount source=/etc/localtime,destination=/etc/localtime,type=bind,readonly \
	 --mount "source=$host_zeek_logs,destination=/usr/local/zeek/logs/,type=bind" \
	 --mount "source=$host_zeek_node_cfg,destination=/usr/local/zeek/etc/node.cfg,type=bind" \
	 "$IMAGE_NAME"

	;;
stop)
	#Command(s) needed to stop the service right now

	if [ "$CONTAINER_RUNNING" == "false" ]; then
		echo "Zeek is already stopped." >&2
		exit 0
	fi

	echo "Stopping the Zeek docker container" >&2
	$SUDO docker stop "$CONTAINER_NAME" -t 90 >&2

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
	$SUDO docker exec "$CONTAINER_NAME" zeekctl status
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

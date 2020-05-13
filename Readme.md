

Original code taken from [blacktop/docker-zeek](https://github.com/blacktop/docker-zeek/tree/master/zeekctl).


# Usage

Edit `etc/node.cfg` to add your capture interface (required) and `docker-compose.yml` to change the log output directory (optional).

## Starting

```
docker-compose up -d
```

## Stopping

```
docker-compose stop -t 90
```

## Updating

```
docker-compose pull
# Don't forget to restart after
```

## Installing a Plugin

```
# Make sure it is running first
docker-compose exec zeek zkg install ja3
```

## Diagnosing Issues

If Zeek crashes right after starting you can check the log output.

```
docker-compose logs
```

If Zeek is successfully capturing and you want to see if there are any issues:

```
# Make sure Zeek is running first
docker-compose exec zeek zeekctl doctor
```


Original code taken from [blacktop/docker-zeek](https://github.com/blacktop/docker-zeek/tree/master/zeekctl).


# Usage

Edit `etc/node.cfg` and optionally `docker-compose.yml`.

## Starting

```
docker-compose up -d
```

## Stopping

```
docker-compose stop -t 90
```

## Installing a Plugin

```
docker-compose exec zeek zkg install ja3
```

## Diagnosing Capture Issues

```
docker-compose exec zeek zeekctl doctor
```
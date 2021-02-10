
[activecm/zeek](https://hub.docker.com/r/activecm/zeek) is meant to run a single-system Zeek cluster inside of a docker container. It is based on, but differs from [blacktop/zeek:zeekctl](https://hub.docker.com/r/blacktop/zeek) in that it focuses on running multiple Zeek processes with `zeekctl`. To that end, there are several helpful features included:

- A [configuration wizard](https://github.com/activecm/zeekcfg) for generating a `node.cfg` cluster configuration
- Will automatically run `zeekctl` on start and print a diagnostic report if it fails
- Cron will periodically ensure that all Zeek processes are running and restart any that have crashed
- Zeek's package maanger is included, allowing you to easily install zeek plugins
- Performance improvement by using `ethtool` to disable certain interface features by default
- Performance improvement with AF_Packet plugin installed and enabled by default in the configuration wizard
- Comes with the following other plugins pre-installed
  - bro-interface-setup 
  - bro-doctor 
  - ja3

## Supported Docker Tags

The docker tags correspond with the version of [Zeek](https://zeek.org/get-zeek/) installed in the image. Zeek currently has two release tracks: feature and lts.

* `latest`, `3.2`, `3.2.3`
* `lts`, `3`, `3.0`, `3.0.12`

## Quickstart

You'll first need Docker. If you don't already have it here is a quick and dirty way to install it on Linux:

```bash
curl -fsSL https://get.docker.com | sh -
```

Otherwise, follow the [install instructions](https://docs.docker.com/get-docker/) for your operating system.

You can then use the `zeek` script in this repo to quickly get Zeek running. We recommend putting this `zeek` script in your system `PATH`. The rest of this readme will assume this repo's `zeek` script is in the system `PATH`.

```bash
sudo wget -O /usr/local/bin/zeek https://raw.githubusercontent.com/activecm/docker-zeek/master/zeek
sudo chmod +x /usr/local/bin/zeek
```

Then use the script to start Zeek.

```bash
zeek start
```

## Customizing

If the Quickstart section above doesn't fit your needs, you can use the following documentation to customize your install.

### Zeek Files Location

The default location our `zeek` script puts its files on your host is `/opt/zeek/`. You can change this directory by setting the `zeek_top_dir` environment variable. We recommend making this change permanent by creating the file `/etc/profile.d/zeek`. For example, to change the directory to `/usr/local/zeek/`:

```bash
echo "export zeek_top_dir=/usr/local/zeek/" | sudo tee -a /etc/profile.d/zeek
source /etc/profile.d/zeek
```

### Zeek Version

The default version tag is `3.0` which will correspond to the latest release in the 3.0 Zeek release channel. You can customize this with the `zeek_release` environment variable. Set this variable to your desired Docker image tag. For example, to use the latest feature release:

```bash
echo "export zeek_release=latest" | sudo tee -a /etc/profile.d/zeek
source /etc/profile.d/zeek
```

### Install a Plugin

You can install Zeek packages from https://packages.zeek.org/ using the Zeek Package Manager, `zkg`. For example, to install the `hassh` plugin:

```
# Run `zeek start` if you haven't already
docker exec -it zeek zkg install hassh
# Restart Zeek to activate plugin
zeek restart
```

Note: Currently only plugins that don't require compiling can be installed.

### Zeek Scripts and local.zeek

You can add custom plugins or scripts by placing your custom files in the appropriate place in Zeek's directory structure on your host system and restarting Zeek. By default these files should be in `/opt/zeek/share/`. For instance, if you have a custom `local.zeek` file you want to use:

```bash
sudo mkdir -p /opt/zeek/share/zeek/site/
sudo mv local.zeek /opt/zeek/share/zeek/site/local.zeek
zeek restart
```

### Zeekctl Config

Zeekctl has several config files you may want to modify such as `zeekctl.cfg` or `networks.cfg`. The default files used are [here](https://github.com/activecm/docker-zeek/tree/master/etc). If you want to provide your own, place your custom file in the appropriate place on your host and then restart Zeek. By default this would be in `/opt/zeek/etc/`.

The `zeek` script will automatically prompt and create a `node.cfg` file for you. If you would like to re-run this prompt you can delete the existing `node.cfg` file and restart Zeek. For instance, if your files are in the default location:

```bash
zeek stop
sudo rm /opt/zeek/etc/node.cfg
zeek start
```

### Updating

You can obtain the newest version of the `zeek` script from this repo.

```bash
sudo wget -O /usr/local/bin/zeek https://raw.githubusercontent.com/activecm/docker-zeek/master/zeek
```

You can use the included `zeek` script to pull the most recent Docker image. This will also restart your Zeek instance.

```bash
zeek update
```

### Diagnosing Issues

If Zeek crashes right after starting you can check the log output.

```
docker logs zeek
```

If Zeek is successfully capturing and you want to see if there are any issues:

```
# Container must be running already
docker exec zeek zeekctl doctor
```

## Development

Developer documentation can be found in the [docs](docs/) folder.

## Credits

Dockerfile based on [blacktop/docker-zeek](https://github.com/blacktop/docker-zeek/tree/master/zeekctl).

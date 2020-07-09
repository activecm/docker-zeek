# Automatic Build (Dockerhub)

The steps to take vary based on what has changed in the new version.

If the Zeek version changes it needs to be updated in the following places:
- Default value for the `ZEEK_VERSION` build arg in the `Dockerfile`
- List of available tags in `Readme.md`
- Version specified in the Github workflow (`.github/workflows/docker.yml`)

If the `Readme.md` changes the contents need to be copied to the Dockerhub project manually. This is due to using Github Actions to push up multiple images (vs. using Dockerhub to pull the code and build a single image). Dockerhub does not automatically update the project with the readme when using the push model. An API is not currently available to do this programmatically.

To trigger a new image build on Dockerhub, push changes to master (or merge a pull request into master) on Github.

# Manual Build

Using default values defined in the dockerfile:

```bash
docker build -t activecm/zeek .
```

Using a specific Zeek version:

```bash
# Note: tag the image with the Zeek version used
docker build --build-arg ZEEK_VERSION=3.0.6 -t activecm/zeek:3.0.6 .
```

Using a specific Zeekcfg version:

```bash
docker build --build-arg ZEEKCFG_VERSION=0.0.4 -t activecm/zeek .
```

Bundling custom Zeek packages in the image:

```bash
docker build --build-arg ZEEK_DEFAULT_PACKAGES="bro-interface-setup ja3 hassh" -t activecm/zeek .
```

# Checking Versions

Verifying the Zeek version installed:

```bash
docker run --rm activecm/zeek zeek --version
```

Verifying the Zeek packages installed:

```bash
docker run --rm activecm/zeek zkg list
```
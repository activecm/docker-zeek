ARG ZEEK_VERSION=6.2.1
ARG ZEEK_DEFAULT_PACKAGES="bro-interface-setup bro-doctor ja3 zeek-open-connections"
ARG ZEEKCFG_VERSION=0.0.5

ARG TARGETPLATFORM 

# The official docker zeek images do not support armv7, so we will build our own
# TODO: build armv7 on bookworm-slim
FROM --platform=linux/arm/v7 alpine AS build-arm
ARG ZEEK_VERSION
ARG BUILD_PROCS=4

RUN apk add --no-cache zlib openssl libstdc++ libpcap libgcc
RUN apk add --no-cache -t .build-deps \
    bsd-compat-headers \
    libmaxminddb-dev \
    linux-headers \
    openssl-dev \
    libpcap-dev \
    python3-dev \
    zlib-dev \
    flex-dev \
    binutils \
    fts-dev \
    cmake \
    bison \
    bash \
    swig \
    perl \
    make \
    flex \
    git \
    gcc \
    g++ \
    fts \
    krb5-dev


RUN echo "===> Cloning zeek..." \
    && cd /tmp \
    && git clone --recursive --branch v$ZEEK_VERSION https://github.com/zeek/zeek.git

RUN echo "===> Compiling zeek..." \
    && cd /tmp/zeek \
    && CC=gcc ./configure --prefix=/usr/local/zeek \
    --build-type=Release \
    --disable-broker-tests \
    --disable-auxtools \
    --disable-javascript \
    && make -j $BUILD_PROCS \
    && make install

RUN echo "===> Shrinking image..." \
    && strip -s /usr/local/zeek/bin/zeek

RUN echo "===> Size of the Zeek install..." \
    && du -sh /usr/local/zeek

# use official zeek images as a base
FROM --platform=linux/amd64 zeek/zeek:$ZEEK_VERSION-amd64 AS build-amd64
FROM --platform=linux/arm64 zeek/zeek:$ZEEK_VERSION-arm64 AS build-arm64

FROM build-$TARGETARCH AS builder
# ####################################################################################################
# INSTALL APT PACKAGES, ZEEK PACKAGES, AND ZKG/ZEEKCFG
# ####################################################################################################
# Previously docker-zeek was based off of alpine, but since the official Zeek docker images are based off of Debian, 
# we will use Debian as well. This will allow us to use the same base image as the official Zeek images for arm64 and amd64, saving a 
# significant amount of time in the build process. 
# Running the binaries from the official Docker Zeek image in Alpine causes issues.
FROM debian:bookworm-slim
ARG ZEEKCFG_VERSION
ARG TARGETARCH
ARG TARGETPLATFORM
ARG ZEEK_VERSION 
ARG ZEEK_DEFAULT_PACKAGES


RUN apt-get -q update \
 && apt-get install -q -y --no-install-recommends \
     ca-certificates net-tools cron \
     git \
     jq \
     libmaxminddb0 \
     libnode108 \
     libpython3.11 \
     libpcap0.8 \
     libssl3 \
     libuv1 \
     libz1 \
     python3-minimal \
     python3-git \
     python3-semantic-version \
     python3-websocket \
     wget \
     ethtool \
     util-linux 


#  TODO: This does not work on the official images, determine if it is needed for armv7
# RUN ln -s $(which ethtool) /sbin/ethtool

# Copy Zeek binaries from the builder image
COPY --from=builder /usr/local/zeek/ /usr/local/zeek/

# Export paths
ENV ZEEKPATH=.:/usr/local/zeek/share/zeek:/usr/local/zeek/share/zeek/policy:/usr/local/zeek/share/zeek/site
ENV PATH=$PATH:/usr/local/zeek/bin

# Install Zeek package manager
# In Zeek v4, zkg is bundled with Zeek. However, the configuration of zkg when bundled with Zeek
# differs from the configuration when installed via pip. The state directory is
# /usr/local/zeek/var/lib/zkg when using v4's bundled zkg. When zkg is installed via pip
# or the --user flag is supplied to the bundled zkg, .root/zkg is used as the state directory.
# In order to re-use the same configuration across v3 and v4, we manually install zkg from pip.
ARG ZKG_VERSION=3.0.1

# TODO: verify that zkg is installed on armv7
# install zkg on armv7 only, since the official zeek images include it 
RUN if [ "$TARGETPLATFORM" == "linux/arm/v7" ]; then pip install --break-system-packages zkg==$ZKG_VERSION ; fi
# TODO: does this work on armv7? using the official images, autoconfig doesn't work without the --force flag
RUN zkg autoconfig --force 
RUN zkg refresh 
RUN zkg install --force $ZEEK_DEFAULT_PACKAGES

# Set TARGET_ARCH to Docker build host arch unless TARGETARCH is specified via BuildKit
RUN case `uname -m` in \
    x86_64) \
        TARGET_ARCH="amd64" \
        ;; \
    aarch64) \
        TARGET_ARCH="arm64" \ 
        ;; \
    arm|armv7l) \
        TARGET_ARCH="arm" \
        ;; \
    esac; \
    TARGET_ARCH=${TARGETARCH:-$TARGET_ARCH}; \
    echo https://github.com/activecm/zeekcfg/releases/download/v${ZEEKCFG_VERSION}/zeekcfg_${ZEEKCFG_VERSION}_linux_${TARGET_ARCH}; \
    wget -qO /usr/local/zeek/bin/zeekcfg https://github.com/activecm/zeekcfg/releases/download/v${ZEEKCFG_VERSION}/zeekcfg_${ZEEKCFG_VERSION}_linux_${TARGET_ARCH} \
    && chmod +x /usr/local/zeek/bin/zeekcfg

# Run zeekctl cron to heal processes every 5 minutes
RUN mkdir -p /etc/crontabs
RUN echo "*/5       *       *       *       *       /usr/local/zeek/bin/zeekctl cron" >> /etc/crontabs/root
COPY docker-entrypoint.sh /docker-entrypoint.sh

# Users must supply their own node.cfg
RUN rm -f /usr/local/zeek/etc/node.cfg
COPY etc/networks.cfg /usr/local/zeek/etc/networks.cfg
COPY etc/zeekctl.cfg /usr/local/zeek/etc/zeekctl.cfg
COPY share/zeek/site/ /usr/local/zeek/share/zeek/site/

CMD ["/docker-entrypoint.sh"]

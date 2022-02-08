FROM alpine:3.12 as builder

ARG ZEEK_VERSION=4.2.0
ARG AF_PACKET_VERSION=3.0.2

ARG BUILD_PROCS=2

RUN apk add --no-cache zlib openssl libstdc++ libpcap libgcc
RUN apk add --no-cache -t .build-deps \
    bsd-compat-headers \
    libmaxminddb-dev \
    linux-headers \
    openssl-dev \
    libpcap-dev \
    python3-dev \
    zlib-dev \
    binutils \
    fts-dev \
    cmake \
    clang \
    bison \
    bash \
    swig \
    perl \
    make \
    flex \
    git \
    g++ \
    fts \
    krb5-dev

RUN echo "===> Cloning zeek..." \
    && cd /tmp \
    && git clone --recursive --branch v$ZEEK_VERSION https://github.com/zeek/zeek.git

RUN echo "===> Compiling zeek..." \
    && cd /tmp/zeek \
    && CC=clang ./configure --prefix=/usr/local/zeek \
    --build-type=Release \
    --disable-broker-tests \
    --disable-auxtools \
    && make -j $BUILD_PROCS \
    && make install

RUN echo "===> Compiling af_packet plugin..." \
    && git clone https://github.com/J-Gras/zeek-af_packet-plugin.git --branch ${AF_PACKET_VERSION} /tmp/zeek-af_packet-plugin \
    && cd /tmp/zeek-af_packet-plugin \
    && CC=clang ./configure --with-kernel=/usr --zeek-dist=/tmp/zeek \
    && make -j $BUILD_PROCS \
    && make install \
    && /usr/local/zeek/bin/zeek -NN Zeek::AF_Packet

RUN echo "===> Shrinking image..." \
    && strip -s /usr/local/zeek/bin/zeek

RUN echo "===> Size of the Zeek install..." \
    && du -sh /usr/local/zeek

####################################################################################################
FROM alpine:3.12

# python3 & bash are needed for zeekctl scripts
# ethtool is needed to manage interface features
# util-linux provides taskset command needed to pin CPUs
# py3-pip and git are needed for zeek's package manager
RUN apk --no-cache add \
    ca-certificates zlib openssl libstdc++ libpcap libmaxminddb libgcc fts krb5-libs \
    python3 bash \
    ethtool \
    util-linux \
    py3-pip git

RUN ln -s $(which ethtool) /sbin/ethtool

COPY --from=builder /usr/local/zeek /usr/local/zeek

ENV ZEEKPATH .:/usr/local/zeek/share/zeek:/usr/local/zeek/share/zeek/policy:/usr/local/zeek/share/zeek/site
ENV PATH $PATH:/usr/local/zeek/bin

# Install Zeek package manager
# In Zeek v4, zkg is bundled with Zeek. However, the configuration of zkg when bundled with Zeek
# differs from the configuration when installed via pip. The state directory is
# /usr/local/zeek/var/lib/zkg when using v4's bundled zkg. When zkg is installed via pip
# or the --user flag is supplied to the bundled zkg, .root/zkg is used as the state directory.
# In order to re-use the same configuration across v3 and v4, we manually install zkg from pip.
ARG ZKG_VERSION=2.12.0

ARG ZEEK_DEFAULT_PACKAGES="bro-interface-setup bro-doctor ja3"

RUN pip install zkg==$ZKG_VERSION \
    && zkg autoconfig \
    && zkg refresh \
    && zkg install --force $ZEEK_DEFAULT_PACKAGES

ARG ZEEKCFG_VERSION=0.0.5

RUN wget -qO /usr/local/zeek/bin/zeekcfg https://github.com/activecm/zeekcfg/releases/download/v${ZEEKCFG_VERSION}/zeekcfg_${ZEEKCFG_VERSION}_linux_amd64 \
    && chmod +x /usr/local/zeek/bin/zeekcfg
# Run zeekctl cron to heal processes every 5 minutes
RUN echo "*/5       *       *       *       *       /usr/local/zeek/bin/zeekctl cron" >> /etc/crontabs/root
COPY docker-entrypoint.sh /docker-entrypoint.sh

# Users must supply their own node.cfg
RUN rm -f /usr/local/zeek/etc/node.cfg
COPY etc/networks.cfg /usr/local/zeek/etc/networks.cfg
COPY etc/zeekctl.cfg /usr/local/zeek/etc/zeekctl.cfg
COPY share/zeek/site/ /usr/local/zeek/share/zeek/site/

CMD ["/docker-entrypoint.sh"]

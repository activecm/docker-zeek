FROM alpine:3.13 as builder

ARG ZEEK_VERSION=4.0.0
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
  fts

RUN echo "===> Cloning zeek..." \
  && cd /tmp \
  && git clone --recursive --branch v$ZEEK_VERSION https://github.com/zeek/zeek.git

RUN echo "===> Compiling zeek..." \
  && cd /tmp/zeek \
  && CC=clang ./configure --prefix=/usr/local/zeek \
  --build-type=MinSizeRel \
  --disable-broker-tests \
  --disable-auxtools \
  && make -j $BUILD_PROCS \
  && make install

RUN echo "===> Compiling af_packet plugin..." \
  && cd /tmp/zeek/auxil/ \
  && git clone https://github.com/J-Gras/zeek-af_packet-plugin.git \
  && cd /tmp/zeek/auxil/zeek-af_packet-plugin \
  && CC=clang ./configure --with-kernel=/usr --zeek-dist=/tmp/zeek \
  && make -j $BUILD_PROCS \
  && make install \
  && /usr/local/zeek/bin/zeek -NN Zeek::AF_Packet

ENV ZEEKPATH .:/usr/local/zeek/share/zeek:/usr/local/zeek/share/zeek/policy:/usr/local/zeek/share/zeek/site
ENV PATH $PATH:/usr/local/zeek/bin

RUN apk add --no-cache py3-pip \
 && python3 -m pip install GitPython semantic-version

ARG ZEEK_DEFAULT_PACKAGES="corelight/zeek-community-id corelight/zeek-globload ncsa/bro-interface-setup ncsa/bro-doctor salesforce/ja3 salesforce/hassh"
# configure Zeek package manager
RUN zkg autoconfig --force \
    && zkg refresh \
    && zkg install --force $ZEEK_DEFAULT_PACKAGES

RUN echo "===> Shrinking image..." \
  && strip -s /usr/local/zeek/bin/zeek

RUN echo "===> Size of the Zeek install..." \
  && du -sh /usr/local/zeek
####################################################################################################
FROM alpine:3.13

# python3 & bash are needed for zeekctl scripts
# ethtool is needed to manage interface features
# util-linux provides taskset command needed to pin CPUs
# py3-pip and git are needed for zeek's package manager
RUN apk --no-cache add \
    ca-certificates zlib openssl libstdc++ libpcap libmaxminddb libgcc fts \
    python3 bash \
    ethtool \
    util-linux \
    py3-pip git
# these are needed for zkg
RUN python3 -m pip install GitPython semantic-version

RUN ln -s $(which ethtool) /sbin/ethtool

COPY --from=builder /usr/local/zeek /usr/local/zeek

ENV ZEEKPATH .:/usr/local/zeek/share/zeek:/usr/local/zeek/share/zeek/policy:/usr/local/zeek/share/zeek/site
ENV PATH $PATH:/usr/local/zeek/bin

# configure Zeek package manager
RUN zkg autoconfig --force \
    && zkg refresh

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

WORKDIR /usr/local/zeek/
CMD ["/docker-entrypoint.sh"]

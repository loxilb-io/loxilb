# Download base image ubuntu 20.04
FROM ubuntu:20.04

# LABEL about the loxilb image
LABEL description="This is loxilb official Docker Image"

# Disable Prompt During Packages Installation
ARG DEBIAN_FRONTEND=noninteractive

# Update Ubuntu Software repository
RUN apt update
RUN apt install -y wget

# Install arch specific packages - golang
RUN arch=$(arch | sed s/aarch64/arm64/ | sed s/x86_64/amd64/) && echo https://go.dev/dl/go1.18.linux-${arch}.tar.gz && wget https://go.dev/dl/go1.18.linux-${arch}.tar.gz && tar -xzf go1.18.linux-${arch}.tar.gz --directory /usr/local/ && rm go1.18.linux-${arch}.tar.gz
ENV PATH="${PATH}:/usr/local/go/bin"

# Install arch specific packages - gcc-multilib
RUN arch=$(arch | sed s/aarch64/arm64/ | sed s/x86_64/amd64/) && echo $arch && if [ "$arch" = "arm64" ] ; then apt install -y gcc-multilib-arm-linux-gnueabihf; else apt install -y  gcc-multilib;fi

# Install loxilb related packages
RUN apt install -y clang llvm libelf-dev libpcap-dev vim net-tools \
    elfutils dwarves git libbsd-dev bridge-utils wget arping unzip build-essential \
    bison flex sudo iproute2 pkg-config tcpdump iputils-ping keepalived curl && \
    rm -rf /var/lib/apt/lists/* && \
    apt clean

RUN wget https://github.com/loxilb-io/iproute2/archive/refs/heads/main.zip && \
    unzip main.zip && cd iproute2-main/libbpf/src/ && mkdir build && \
    DESTDIR=build make install && cd - && \
    cd iproute2-main/ &&  export PKG_CONFIG_PATH=$PKG_CONFIG_PATH:`pwd`/libbpf/src/ && LIBBPF_FORCE=on LIBBPF_DIR=`pwd`/libbpf/src/build ./configure && make && cp -f tc/tc /usr/local/sbin/ntc && cd - && cd iproute2-main/libbpf/src/ && make install && cd - && rm -fr main.zip iproute2-main

# Build bpftool
RUN git clone --recurse-submodules https://github.com/libbpf/bpftool.git && cd bpftool/src/ && make clean && 	make -j $(nproc) && cp -f ./bpftool /usr/local/sbin/bpftool && cd - && rm -fr bpftool

# Install loxicmd
RUN git clone https://github.com/loxilb-io/loxicmd.git && cd loxicmd && go get . && make && cp ./loxicmd /usr/local/sbin/loxicmd && cd - && rm -fr loxicmd

# Install gobgpd
RUN wget https://github.com/osrg/gobgp/releases/download/v3.5.0/gobgp_3.5.0_linux_amd64.tar.gz && tar -xzf gobgp_3.5.0_linux_amd64.tar.gz &&  mv gobgp* /usr/sbin/ && rm LICENSE README.md

# Make loxilb eBPF filesystem dir
RUN mkdir -p /opt/loxilb
RUN mkdir -p /opt/loxilb/cert/
RUN mkdir -p /root/loxilb-io/loxilb/

# Install loxilb
RUN git clone --recurse-submodules https://github.com/loxilb-io/loxilb  /root/loxilb-io/loxilb/ && cd /root/loxilb-io/loxilb/ && go get . && make && cp loxilb-ebpf/utils/mkllb_bpffs.sh /usr/local/sbin/mkllb_bpffs && cp api/certification/* /opt/loxilb/cert/ && cd -
RUN cp /root/loxilb-io/loxilb/loxilb /usr/local/sbin/loxilb
RUN rm -fr /root/loxilb-io/loxilb/*
RUN rm -fr /root/loxilb-io/loxilb/.git
RUN rm -fr /root/loxilb-io/loxilb/.github
RUN mkdir -p /root/loxilb-io/loxilb/
RUN cp /usr/local/sbin/loxilb /root/loxilb-io/loxilb/loxilb
#RUN /usr/local/sbin/mkllb_bpffs

#RUN cd /root/loxilb-io/loxilb/ && make test
 
ENTRYPOINT ["/root/loxilb-io/loxilb/loxilb"]

# Expose Ports
EXPOSE 11111 22222

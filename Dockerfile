# Download base image ubuntu 20.04
FROM ubuntu:20.04

# LABEL about the loxilb image
LABEL description="This is loxilb official Docker Image"

# Disable Prompt During Packages Installation
ARG DEBIAN_FRONTEND=noninteractive

# Prepare environment
RUN mkdir -p /opt/loxilb && \
    mkdir -p /opt/loxilb/cert/ && \
    mkdir -p /root/loxilb-io/loxilb/ && \
    mkdir -p /etc/bash_completion.d/

# Update Ubuntu Software repository
RUN apt update && apt install -y wget

# Env for golang
ENV PATH="${PATH}:/usr/local/go/bin"

# Install loxilb related packages
RUN arch=$(arch | sed s/aarch64/arm64/ | sed s/x86_64/amd64/) && echo $arch && if [ "$arch" = "arm64" ] ; then apt install -y gcc-multilib-arm-linux-gnueabihf; else apt update && apt install -y  gcc-multilib;fi && \
    # Arch specific packages - GoLang
    wget https://go.dev/dl/go1.18.linux-${arch}.tar.gz && tar -xzf go1.18.linux-${arch}.tar.gz --directory /usr/local/ && rm go1.18.linux-${arch}.tar.gz && \
    # Dev and util packages
    apt install -y clang llvm libelf-dev libpcap-dev vim net-tools \
    elfutils dwarves git libbsd-dev bridge-utils wget unzip build-essential \
    bison flex sudo iproute2 pkg-config tcpdump iputils-ping keepalived curl bash-completion && \
    # Install loxilb's custom ntc tool
    wget https://github.com/loxilb-io/iproute2/archive/refs/heads/main.zip && \
    unzip main.zip && cd iproute2-main/libbpf/src/ && mkdir build && \
    DESTDIR=build make install && cd - && cd iproute2-main/ && \
    export PKG_CONFIG_PATH=$PKG_CONFIG_PATH:`pwd`/libbpf/src/ && \
    LIBBPF_FORCE=on LIBBPF_DIR=`pwd`/libbpf/src/build ./configure && make && \
    cp -f tc/tc /usr/local/sbin/ntc && cd - && cd iproute2-main/libbpf/src/ && \
    make install && cd - && rm -fr main.zip iproute2-main && \
    # Install bpftool
    git clone --recurse-submodules https://github.com/libbpf/bpftool.git && cd bpftool/src/ && \
    make clean && 	make -j $(nproc) && cp -f ./bpftool /usr/local/sbin/bpftool && \
    cd - && rm -fr bpftool && \
    # Install loxicmd
    git clone https://github.com/loxilb-io/loxicmd.git && cd loxicmd && go get . && \
    make && cp ./loxicmd /usr/local/sbin/loxicmd && cd - && rm -fr loxicmd && \
    /usr/local/sbin/loxicmd completion bash > /etc/bash_completion.d/loxi_completion && \
    # Install loxilb
    git clone --recurse-submodules https://github.com/loxilb-io/loxilb  /root/loxilb-io/loxilb/ && \
    cd /root/loxilb-io/loxilb/ && go get . && make && \
    cp loxilb-ebpf/utils/mkllb_bpffs.sh /usr/local/sbin/mkllb_bpffs && \
    cp api/certification/* /opt/loxilb/cert/ && cd - && \
    cp /root/loxilb-io/loxilb/loxilb-ebpf/kernel/loxilb_dp_debug  /usr/local/sbin/loxilb_dp_debug && \
    cp /root/loxilb-io/loxilb/loxilb /usr/local/sbin/loxilb && \
    rm -fr /root/loxilb-io/loxilb/* && rm -fr /root/loxilb-io/loxilb/.git && \
    rm -fr /root/loxilb-io/loxilb/.github && mkdir -p /root/loxilb-io/loxilb/ && \
    cp /usr/local/sbin/loxilb /root/loxilb-io/loxilb/loxilb && rm /usr/local/sbin/loxilb && \
    # Install gobgp
    wget https://github.com/osrg/gobgp/releases/download/v3.5.0/gobgp_3.5.0_linux_amd64.tar.gz && \
    tar -xzf gobgp_3.5.0_linux_amd64.tar.gz &&  rm gobgp_3.5.0_linux_amd64.tar.gz && \
    mv gobgp* /usr/sbin/ && rm LICENSE README.md && \
    apt-get purge -y clang llvm libelf-dev libpcap-dev libbsd-dev build-essential \
    elfutils dwarves git bison flex curl wget unzip && apt-get -y autoremove && \
    apt-get install -y libllvm10 && \
    # cleanup unnecessary packages
    if [ "$arch" = "arm64" ] ; then apt purge -y gcc-multilib-arm-linux-gnueabihf; else apt update && apt purge -y gcc-multilib;fi && \
    rm -rf /var/lib/apt/lists/* && apt clean && \
    echo "if [ -f /etc/bash_completion ] && ! shopt -oq posix; then" >> /root/.bashrc && \
    echo "    . /etc/bash_completion" >> /root/.bashrc && \
    echo "fi" >> /root/.bashrc

## Please note that bpftool needs llvm for debugging.
## We need to reinstall llvm while debugging

# Optional files, only apply when files exist
COPY ./loxilb.rep* /root/loxilb-io/loxilb/loxilb
COPY ./llb_ebpf_main.o.rep* /opt/loxilb/llb_ebpf_main.o
COPY ./llb_xdp_main.o.rep* /opt/loxilb/llb_xdp_main.o

ENTRYPOINT ["/root/loxilb-io/loxilb/loxilb"]

# Expose Ports
EXPOSE 11111 22222

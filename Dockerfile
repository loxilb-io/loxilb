# Download base image ubuntu 20.04
FROM ubuntu:20.04

# LABEL about the loxilb image
LABEL description="This is loxilb official Docker Image"

# Disable Prompt During Packages Installation
ARG DEBIAN_FRONTEND=noninteractive

# Update Ubuntu Software repository
RUN apt update

# Install loxilb related packages
RUN apt install -y clang llvm libelf-dev gcc-multilib libpcap-dev vim net-tools \
    linux-tools-$(uname -r) elfutils dwarves git libbsd-dev bridge-utils wget \
    unzip build-essential bison flex sudo iproute2 pkg-config && \
    rm -rf /var/lib/apt/lists/* && \
    apt clean

# Install GoLang
RUN wget https://go.dev/dl/go1.18.linux-amd64.tar.gz && tar -xzf go1.18.linux-amd64.tar.gz --directory /usr/local/
ENV PATH="${PATH}:/usr/local/go/bin"
RUN rm go1.18.linux-amd64.tar.gz

RUN wget https://github.com/loxilb-io/iproute2/archive/refs/heads/main.zip && \
    unzip main.zip && cd iproute2-main/libbpf/src/ && mkdir build && \
    DESTDIR=build make install && cd - && \
    cd iproute2-main/ &&  export PKG_CONFIG_PATH=$PKG_CONFIG_PATH:`pwd`/libbpf/src/ && LIBBPF_FORCE=on LIBBPF_DIR=`pwd`/libbpf/src/build ./configure && make && cp -f tc/tc /usr/local/sbin/ntc && cd - && cd iproute2-main/libbpf/src/ && make install && cd - 

# Install loxicmd
RUN git clone https://github.com/loxilb-io/loxicmd.git && cd loxicmd && go get . && make && cp ./loxicmd /usr/local/sbin/loxicmd && cd - && rm -fr loxicmd

# Install gobgpd
RUN wget https://github.com/osrg/gobgp/releases/download/v3.5.0/gobgp_3.5.0_linux_amd64.tar.gz && tar -xzf gobgp_3.5.0_linux_amd64.tar.gz &&  mv gobgp* /usr/sbin/ && rm LICENSE README.md

# Make loxilb eBPF filesystem dir
RUN mkdir -p /opt/loxilb
RUN mkdir -p /opt/loxilb/cert/
RUN mkdir -p /root/loxilb-io/loxilb/

# Install loxilb
RUN git clone https://github.com/loxilb-io/loxilb  /root/loxilb-io/loxilb/ && cd /root/loxilb-io/loxilb/ && go get . && make && cp ebpf/utils/mkllb_bpffs.sh /usr/local/sbin/mkllb_bpffs && cp api/certification/* /opt/loxilb/cert/ && cd -
#RUN /usr/local/sbin/mkllb_bpffs

#RUN cd /root/loxilb-io/loxilb/ && make test
 
ENTRYPOINT ["/root/loxilb-io/loxilb/loxilb"]

# Expose Port for loxicmd
EXPOSE 11111

.DEFAULT_GOAL := build
bin=loxilb
dock?=loxilb
IMAGE?=ghcr.io/loxilb-io/loxilb
TAG?=latest
ARM64_TAG?=$(TAG)-arm64
BRANCH_NAME:=$(shell git rev-parse --is-inside-work-tree >/dev/null 2>&1 && git branch --show-current || echo nogit)

# VERSION stamps the release version into common.Version. The git tag is the
# source of truth for it -- the source tree carries only a dev placeholder -- so
# this defaults to the tag when building from an exact tag checkout (which is how
# the deb/qcow2 packaging builds work) and is empty otherwise. Pass it explicitly
# with `make VERSION=x.y.z` where no tag is reachable, as the image builds do.
# When empty no stamp is applied and the placeholder in common/common.go shows
# through, so a plain `go build` or a branch build still works unchanged.
VERSION?=$(shell git describe --tags --exact-match 2>/dev/null)
# `override` so that a leading v is stripped even when VERSION came from the
# command line, which a plain assignment cannot do.
override VERSION:=$(patsubst v%,%,$(VERSION))
LDFLAGS:=-X 'github.com/loxilb-io/loxilb/common.BuildInfo=$(shell date '+%Y_%m_%d_%Hh:%Mm')-$(BRANCH_NAME)'
ifneq ($(VERSION),)
LDFLAGS+= -X 'github.com/loxilb-io/loxilb/common.Version=$(VERSION)'
endif

loxilbid=$(shell docker ps -f name=$(dock) | grep -w $(dock) | cut  -d " "  -f 1 | grep -iv  "CONTAINER")

subsys:
	cd loxilb-ebpf && $(MAKE) 

subsys-clean:
	cd loxilb-ebpf && $(MAKE) clean

build: subsys
	@go build -o ${bin} -ldflags="$(LDFLAGS)"
	
clean: subsys-clean
	go clean

test:
	go test .

check:
	go test .

run:
	./$(bin)

docker-cp: build
	docker cp loxilb $(loxilbid):/root/loxilb-io/loxilb/loxilb
	docker cp loxilb-ebpf/kernel/llb_ebpf_main.o $(loxilbid):/opt/loxilb/llb_ebpf_main.o
	docker cp loxilb-ebpf/kernel/llb_ebpf_emain.o $(loxilbid):/opt/loxilb/llb_ebpf_emain.o
	docker cp loxilb-ebpf/kernel/llb_xdp_main.o $(loxilbid):/opt/loxilb/llb_xdp_main.o
	docker cp loxilb-ebpf/kernel/llb_kern_sock.o $(loxilbid):/opt/loxilb/llb_kern_sock.o
	docker cp loxilb-ebpf/kernel/llb_kern_sockmap.o $(loxilbid):/opt/loxilb/llb_kern_sockmap.o
	docker cp loxilb-ebpf/kernel/llb_kern_sockstream.o $(loxilbid):/opt/loxilb/llb_kern_sockstream.o
	docker cp loxilb-ebpf/kernel/llb_kern_sockdirect.o $(loxilbid):/opt/loxilb/llb_kern_sockdirect.o
	docker cp loxilb-ebpf/kernel/loxilb_dp_debug  $(loxilbid):/usr/local/sbin/
	docker cp loxilb-ebpf/libbpf/src/libbpf.so.1.5.0 $(loxilbid):/usr/lib64/
	docker cp loxilb-ebpf/utils/loxilb_dp_tool $(loxilbid):/usr/local/sbin/

docker-cp-ebpf: build
	docker cp loxilb-ebpf/kernel/llb_ebpf_main.o $(loxilbid):/opt/loxilb/llb_ebpf_main.o
	docker cp loxilb-ebpf/kernel/llb_ebpf_emain.o $(loxilbid):/opt/loxilb/llb_ebpf_emain.o
	docker cp loxilb-ebpf/kernel/llb_xdp_main.o $(loxilbid):/opt/loxilb/llb_xdp_main.o
	docker cp loxilb-ebpf/kernel/llb_kern_sock.o $(loxilbid):/opt/loxilb/llb_kern_sock.o
	docker cp loxilb-ebpf/kernel/loxilb_dp_debug  $(loxilbid):/usr/local/sbin/
	docker cp loxilb-ebpf/libbpf/src/libbpf.so.1.5.0 $(loxilbid):/usr/lib64/

docker-run:
	@docker stop $(dock) 2>&1 >> /dev/null || true
	@docker rm $(dock) 2>&1 >> /dev/null || true
	docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dt --entrypoint /bin/bash  --name $(dock) $(IMAGE):$(TAG)

docker-rp: docker-run docker-cp
	@docker exec -it $(dock) mkllb_bpffs 2>&1 >> /dev/null || true
	docker commit ${loxilbid} $(IMAGE):$(TAG)
	@docker stop $(dock) 2>&1 >> /dev/null || true
	@docker rm $(dock) 2>&1 >> /dev/null || true

docker-rp-ebpf: docker-run docker-cp-ebpf
	docker commit ${loxilbid} $(IMAGE):$(TAG)
	@docker stop $(dock) 2>&1 >> /dev/null || true
	@docker rm $(dock) 2>&1 >> /dev/null || true

# VERSION is forwarded into the image build because .dockerignore excludes .git,
# so the in-container `make` cannot derive it from a tag the way the deb build
# does. Without it the image falls back to the dev placeholder in common.go.
docker:
	docker build -t $(IMAGE):$(TAG) --build-arg VERSION=$(VERSION) .

docker-arm64:
	docker  buildx build --platform linux/arm64 --load -t $(IMAGE):$(ARM64_TAG) --build-arg VERSION=$(VERSION) .

lint:
	golangci-lint run --enable-all

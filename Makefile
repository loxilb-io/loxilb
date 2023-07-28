.DEFAULT_GOAL := build
bin=loxilb
dock?=loxilb

loxilbid=$(shell docker ps -f name=$(dock) | grep -w $(dock) | cut  -d " "  -f 1 | grep -iv  "CONTAINER")

subsys:
	cd loxilb-ebpf && $(MAKE) 

subsys-clean:
	cd loxilb-ebpf && $(MAKE) clean

build: subsys
	@go build -o ${bin} -ldflags="-X 'main.buildInfo=${shell date '+%Y_%m_%d'}-${shell git branch --show-current}'"

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
	docker cp /opt/loxilb/llb_ebpf_main.o $(loxilbid):/opt/loxilb/llb_ebpf_main.o
	docker cp /opt/loxilb/llb_ebpf_emain.o $(loxilbid):/opt/loxilb/llb_ebpf_emain.o
	docker cp /opt/loxilb/llb_xdp_main.o $(loxilbid):/opt/loxilb/llb_xdp_main.o
	docker cp loxilb-ebpf/kernel/loxilb_dp_debug  $(loxilbid):/usr/local/sbin/

docker-cp-ebpf: build
	docker cp /opt/loxilb/llb_ebpf_main.o $(loxilbid):/opt/loxilb/llb_ebpf_main.o
	docker cp /opt/loxilb/llb_ebpf_emain.o $(loxilbid):/opt/loxilb/llb_ebpf_emain.o
	docker cp /opt/loxilb/llb_xdp_main.o $(loxilbid):/opt/loxilb/llb_xdp_main.o

docker-run:
	@docker stop $(dock) 2>&1 >> /dev/null || true
	@docker rm $(dock) 2>&1 >> /dev/null || true
	docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dt --entrypoint /bin/bash  --name $(dock) ghcr.io/loxilb-io/loxilb:latest

docker-rp: docker-run docker-cp
	docker commit ${loxilbid} ghcr.io/loxilb-io/loxilb:latest
	@docker stop $(dock) 2>&1 >> /dev/null || true
	@docker rm $(dock) 2>&1 >> /dev/null || true

docker-rp-ebpf: docker-run docker-cp-ebpf
	docker commit ${loxilbid} ghcr.io/loxilb-io/loxilb:latest
	@docker stop $(dock) 2>&1 >> /dev/null || true
	@docker rm $(dock) 2>&1 >> /dev/null || true

docker:
	docker build -t ghcr.io/loxilb-io/loxilb:latest .

docker-arm64:
	docker  buildx build --platform linux/arm64 -t ghcr.io/loxilb-io/loxilb:latest-arm64 .

lint:
	golangci-lint run --enable-all

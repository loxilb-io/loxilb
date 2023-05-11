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
	docker cp /opt/loxilb/llb_xdp_main.o $(loxilbid):/opt/loxilb/llb_xdp_main.o

docker-cp-ebpf: build
	docker cp /opt/loxilb/llb_ebpf_main.o $(loxilbid):/opt/loxilb/llb_ebpf_main.o
	docker cp /opt/loxilb/llb_xdp_main.o $(loxilbid):/opt/loxilb/llb_xdp_main.o

docker-rp: build
	cp loxilb ./loxilb.rep
	cp /opt/loxilb/llb_ebpf_main.o ./llb_ebpf_main.o.rep
	cp /opt/loxilb/llb_xdp_main.o ./llb_xdp_main.o.rep
	$(MAKE) docker
	rm ./llb_ebpf_main.o.rep ./llb_xdp_main.o.rep ./loxilb.rep

docker-rp-ebpf: build
	cp /opt/loxilb/llb_ebpf_main.o ./llb_ebpf_main.o.rep
	cp /opt/loxilb/llb_xdp_main.o ./llb_xdp_main.o.rep
	$(MAKE) docker
	rm ./llb_ebpf_main.o.rep ./llb_xdp_main.o.rep

docker:
	docker build -t ghcr.io/loxilb-io/loxilb:latest .

docker-arm64:
	docker  buildx build --platform linux/arm64 -t ghcr.io/loxilb-io/loxilb:latest-arm64 .

lint:
	golangci-lint run --enable-all

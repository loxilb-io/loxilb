.DEFAULT_GOAL := build
bin=loxilb

loxilbid=$(shell docker ps -f name=loxilb | cut  -d " "  -f 1 | grep -iv  "CONTAINER")

subsys:
	cd ebpf && $(MAKE) 

subsys-clean:
	cd ebpf && $(MAKE) clean

subsys-test:
	cd toolkits && go test

build: subsys
	@go build -o ${bin}

clean: subsys-clean
	go clean

test: subsys-test
	go test

check:
	go test

run:
	./$(bin)

docker-cp: build
	docker cp loxilb $(loxilbid):/root/loxilb-io/loxilb/loxilb
	docker cp /opt/loxilb/llb_ebpf_main.o $(loxilbid):/opt/loxilb/llb_ebpf_main.o
	docker cp /opt/loxilb/llb_xdp_main.o $(loxilbid):/opt/loxilb/llb_xdp_main.o

lint:
	golangci-lint run --enable-all

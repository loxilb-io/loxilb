.DEFAULT_GOAL := build
bin=loxilb

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

lint:
	golangci-lint run --enable-all

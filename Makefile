.PHONY: all test testv clean fmt lint
.PHONY: distribute

GOBIN = ./build/bin
GOCMD = env GO111MODULE=on GOPROXY=https://goproxy.io go

distribute:
	$(GOCMD) run build/ci.go install ./cmd/distribute
	@echo "Done building."
	@echo "Run \"$(GOBIN)/distribute\" to launch distribute."

all:
	$(GOCMD) build -v ./...
	$(GOCMD) run build/ci.go install ./cmd/...
	@echo "Done building."
	@echo "Find binaries in \"$(GOBIN)\" directory."
	@echo ""
	@echo "Copy config-example.toml to \"$(GOBIN)\"."
	@cp params/config-example.toml $(GOBIN)

test: all
	$(GOCMD) test ./...

testv: all
	$(GOCMD) test -v ./...

clean:
	$(GOCMD) clean -cache
	rm -fr $(GOBIN)/*

fmt:
	./gofmt.sh

lint:
	golangci-lint run ./...

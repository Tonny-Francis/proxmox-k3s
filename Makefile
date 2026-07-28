BINARY   := proxmox-k3s
CMD      := ./cmd/proxmox-k3s
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test lint clean install

build:
	go build $(LDFLAGS) -o dist/$(BINARY) $(CMD)

build-all:
	GOOS=linux  GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 $(CMD)
	GOOS=linux  GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 $(CMD)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 $(CMD)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 $(CMD)

test:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run ./...

install: build
	install -m 755 dist/$(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -rf dist/ coverage.out

run-validate:
	go run $(CMD) validate -c examples/cluster.ha.yaml --skip-remote

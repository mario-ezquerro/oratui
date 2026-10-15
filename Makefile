BINARY_NAME=ora-tui
VERSION=1.0.0

.PHONY: all build build-linux test preview mock rpm clean

all: test build build-linux

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(BINARY_NAME) ./cmd/ora-tui

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/ora-tui

test:
	go test -v ./...

preview: build
	./bin/$(BINARY_NAME) --preview

mock: build
	./bin/$(BINARY_NAME) --mock --topology=rac

rpm: build-linux
	mkdir -p dist
	go run github.com/goreleaser/nfpm/v2/cmd/nfpm@latest pkg --packager rpm --config packaging/nfpm.yaml --target dist/$(BINARY_NAME)-$(VERSION)-1.x86_64.rpm

clean:
	rm -rf bin/ dist/

BINARY_NAME=ora-tui
VERSION=1.10.0

.PHONY: all build build-linux test preview mock rpm deb packages clean

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
	go run github.com/goreleaser/nfpm/v2/cmd/nfpm@latest pkg --packager rpm --config packaging/nfpm.yaml --target dist/$(BINARY_NAME)-1.10.0-1.x86_64.rpm
	cp dist/$(BINARY_NAME)-1.10.0-1.x86_64.rpm dist/$(BINARY_NAME)-1.9.0-1.x86_64.rpm
	cp dist/$(BINARY_NAME)-1.10.0-1.x86_64.rpm dist/$(BINARY_NAME)-1.8.0-1.x86_64.rpm
	cp dist/$(BINARY_NAME)-1.10.0-1.x86_64.rpm dist/$(BINARY_NAME)-1.7.0-1.x86_64.rpm
	cp dist/$(BINARY_NAME)-1.10.0-1.x86_64.rpm dist/$(BINARY_NAME)-1.6.0-1.x86_64.rpm
	cp dist/$(BINARY_NAME)-1.10.0-1.x86_64.rpm dist/$(BINARY_NAME)-1.5.0-1.x86_64.rpm
	cp dist/$(BINARY_NAME)-1.10.0-1.x86_64.rpm dist/$(BINARY_NAME)-1.4.0-1.x86_64.rpm
	cp dist/$(BINARY_NAME)-1.10.0-1.x86_64.rpm dist/$(BINARY_NAME)-1.3.0-1.x86_64.rpm
	cp dist/$(BINARY_NAME)-1.10.0-1.x86_64.rpm dist/$(BINARY_NAME)-1.0.0-1.x86_64.rpm

deb: build-linux
	mkdir -p dist
	go run github.com/goreleaser/nfpm/v2/cmd/nfpm@latest pkg --packager deb --config packaging/nfpm.yaml --target dist/$(BINARY_NAME)_1.10.0-1_amd64.deb
	cp dist/$(BINARY_NAME)_1.10.0-1_amd64.deb dist/$(BINARY_NAME)_1.9.0-1_amd64.deb
	cp dist/$(BINARY_NAME)_1.10.0-1_amd64.deb dist/$(BINARY_NAME)_1.8.0-1_amd64.deb
	cp dist/$(BINARY_NAME)_1.10.0-1_amd64.deb dist/$(BINARY_NAME)_1.7.0-1_amd64.deb
	cp dist/$(BINARY_NAME)_1.10.0-1_amd64.deb dist/$(BINARY_NAME)_1.6.0-1_amd64.deb
	cp dist/$(BINARY_NAME)_1.10.0-1_amd64.deb dist/$(BINARY_NAME)_1.5.0-1_amd64.deb
	cp dist/$(BINARY_NAME)_1.10.0-1_amd64.deb dist/$(BINARY_NAME)_1.4.0-1_amd64.deb
	cp dist/$(BINARY_NAME)_1.10.0-1_amd64.deb dist/$(BINARY_NAME)_1.3.0-1_amd64.deb
	cp dist/$(BINARY_NAME)_1.10.0-1_amd64.deb dist/$(BINARY_NAME)_1.0.0-1_amd64.deb

packages: rpm deb

clean:
	rm -rf bin/ dist/

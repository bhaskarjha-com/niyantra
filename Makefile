VERSION ?= $(shell cat VERSION 2>/dev/null || echo "dev")
LDFLAGS  = -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build run test vet clean demo

## Build the binary with version injection
build:
	go build $(LDFLAGS) -o niyantra ./cmd/niyantra

## Build and start the dashboard
run: build
	./niyantra serve

## Run all tests with race detection
test:
	go test -race -count=1 ./...

## Run Go vet
vet:
	go vet ./...

## Remove built binaries
clean:
	rm -f niyantra niyantra.exe

## Seed demo data and launch dashboard
demo: build
	./niyantra demo
	./niyantra serve

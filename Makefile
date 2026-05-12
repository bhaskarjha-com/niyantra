VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS  = -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build run test vet lint vulncheck clean demo js js-prod js-watch css css-prod css-watch

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

## Run golangci-lint (includes staticcheck, errcheck, etc.)
lint:
	golangci-lint run ./...

## Run govulncheck for dependency vulnerability scanning
vulncheck:
	govulncheck ./...

## Bundle frontend JS
js:
	npx.cmd -y esbuild internal/web/src/main.ts --bundle --format=iife \
		--outfile=internal/web/static/app.js --target=es2020 \
		--banner:js="// GENERATED FILE — do not edit. Source: internal/web/src/"

## Bundle + minify for production
js-prod:
	npx.cmd -y esbuild internal/web/src/main.ts --bundle --format=iife \
		--outfile=internal/web/static/app.js --target=es2020 --minify \
		--banner:js="// GENERATED FILE — do not edit. Source: internal/web/src/"

## Development: watch + rebuild
js-watch:
	npx.cmd -y esbuild internal/web/src/main.ts --bundle --format=iife \
		--outfile=internal/web/static/app.js --target=es2020 --watch \
		--banner:js="// GENERATED FILE — do not edit. Source: internal/web/src/"

## Remove built binaries
clean:
	rm -f niyantra niyantra.exe

## Seed demo data and launch dashboard
demo: build
	./niyantra demo
	./niyantra serve

## Bundle frontend CSS
css:
	npx.cmd -y esbuild internal/web/src/styles/main.css --bundle \
		--outfile=internal/web/static/style.css

## Bundle + minify CSS for production
css-prod:
	npx.cmd -y esbuild internal/web/src/styles/main.css --bundle --minify \
		--outfile=internal/web/static/style.css

## Development: watch + rebuild CSS
css-watch:
	npx.cmd -y esbuild internal/web/src/styles/main.css --bundle \
		--outfile=internal/web/static/style.css --watch

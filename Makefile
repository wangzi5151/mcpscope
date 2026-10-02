VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build test vet fmt dist clean

build:
	go build -ldflags "$(LDFLAGS)" -o mcpscope ./cmd/mcpscope

test:
	go test ./... -race -count=1

vet:
	go vet ./...

fmt:
	gofmt -l .

# Cross-platform release binaries, named mcpscope-<os>-<arch>[.exe].
dist:
	rm -rf dist && mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/mcpscope-linux-amd64 ./cmd/mcpscope
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/mcpscope-linux-arm64 ./cmd/mcpscope
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/mcpscope-darwin-amd64 ./cmd/mcpscope
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/mcpscope-darwin-arm64 ./cmd/mcpscope
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/mcpscope-windows-amd64.exe ./cmd/mcpscope
	GOOS=windows GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/mcpscope-windows-arm64.exe ./cmd/mcpscope

clean:
	rm -rf dist mcpscope

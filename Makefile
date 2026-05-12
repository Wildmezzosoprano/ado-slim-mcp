.PHONY: build build-all tidy test vet clean

LDFLAGS := -s -w

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -trimpath -o bin/ado-slim-mcp.exe ./cmd/server

build-all:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -trimpath -o bin/ado-slim-mcp.exe ./cmd/server
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -trimpath -o bin/ado-slim-mcp-linux-amd64 ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -trimpath -o bin/ado-slim-mcp-darwin-arm64 ./cmd/server

tidy:
	go mod tidy

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf bin

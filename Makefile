BINARY := agentmux

.PHONY: all build test lint fmt vet clean

all: build test lint

build:
	go build -o bin/$(BINARY) .

test:
	go test ./...

lint:
	golangci-lint run

fmt:
	gofmt -l -w .

vet:
	go vet ./...

clean:
	rm -rf bin/ dist/

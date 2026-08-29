BINARY := jumux

.PHONY: all tools build test lint fmt vet clean

all: tools build test lint

tools:
	mise install

build:
	mise exec -- go build -o bin/$(BINARY) .

test:
	mise exec -- go test ./...

lint:
	mise exec -- golangci-lint run

fmt:
	gofmt -l -w .

vet:
	mise exec -- go vet ./...

clean:
	rm -rf bin/ dist/

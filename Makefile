BINARY := jumux

.PHONY: all tools build test lint fmt vet vulncheck clean

all: tools build test lint vulncheck

tools:
	mise install

build:
	mise exec -- go build -o bin/$(BINARY) .

test:
	mise exec -- go test ./...

lint:
	mise exec -- golangci-lint run

vulncheck:
	mise exec -- govulncheck ./...

fmt:
	gofmt -l -w .

vet:
	mise exec -- go vet ./...

clean:
	rm -rf bin/ dist/

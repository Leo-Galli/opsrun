BINARY := opsrun
PKG := ./...

.PHONY: build test test-race lint vet vuln fmt tidy clean

build:
	go build -o bin/$(BINARY) ./cmd/opsrun

test:
	go test $(PKG)

test-race:
	go test -race -covermode=atomic -coverprofile=coverage.out $(PKG)

vet:
	go vet $(PKG)

lint:
	golangci-lint run

vuln:
	govulncheck $(PKG)

fmt:
	gofmt -l .

tidy:
	go mod tidy

clean:
	rm -rf bin coverage.out

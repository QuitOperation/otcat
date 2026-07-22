VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build test race cover bench fuzz vet fmt lint clean install-tools

build:
	go build -ldflags "$(LDFLAGS)" -o bin/otcat ./cmd/otcat
	go build -ldflags "$(LDFLAGS)" -o bin/otc ./cmd/otc
	go build -ldflags "$(LDFLAGS)" -o bin/otcat-mockplc ./cmd/otcat-mockplc
	go build -ldflags "$(LDFLAGS)" -o bin/otcat-latencyprobe ./cmd/otcat-latencyprobe

test:
	go test ./...

race:
	go test ./... -race -count=1

cover:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -func=coverage.out

bench:
	go test ./... -run=^$$ -bench=. -benchmem -benchtime=1s -count=3

fuzz: ## make fuzz TARGET=FuzzDecodeMBAP DURATION=30s
	go test ./internal/modbus/ -fuzz=^$(TARGET)$$ -fuzztime=$(or $(DURATION),15s) -run=^$$

vet:
	go vet ./...

fmt:
	gofmt -l .

clean:
	rm -rf bin coverage.out

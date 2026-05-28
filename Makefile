BIN := bin
PRQ := $(BIN)/prq
OCM := $(BIN)/ocm

.PHONY: all build prq ocm lint clean install

all: build

build: prq ocm

prq:
	go build -o $(PRQ) ./cmd/prq

ocm:
	go build -o $(OCM) ./cmd/ocm

lint:
	golangci-lint run ./...

install:
	go install ./cmd/prq ./cmd/ocm

clean:
	rm -rf $(BIN)

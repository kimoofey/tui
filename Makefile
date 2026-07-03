BIN := bin
PRQ := $(BIN)/prq
OCM := $(BIN)/ocm
SCREENSHOTS_DIR := screenshots

.PHONY: all build prq ocm lint test clean install screenshots

all: build

build: prq ocm

prq:
	go build -o $(PRQ) ./cmd/prq

ocm:
	go build -o $(OCM) ./cmd/ocm

lint:
	golangci-lint run ./...

test:
	go test ./...

install:
	go install ./cmd/prq ./cmd/ocm

clean:
	rm -rf $(BIN)

screenshots:
	$(eval TAPE_BIN := $(shell mktemp -d))
	go build -tags mock -o $(TAPE_BIN)/prq ./cmd/prq
	go build -tags mock -o $(TAPE_BIN)/ocm ./cmd/ocm
	PATH=$(TAPE_BIN):$$PATH vhs $(SCREENSHOTS_DIR)/prq.tape
	PATH=$(TAPE_BIN):$$PATH vhs $(SCREENSHOTS_DIR)/ocm.tape
	rm -rf $(TAPE_BIN)

BIN := bin
PRQ := $(BIN)/prq
OCM := $(BIN)/ocm
PRQ_MOCK := $(BIN)/prq-mock
OCM_MOCK := $(BIN)/ocm-mock
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

screenshots: $(PRQ_MOCK) $(OCM_MOCK)
	vhs $(SCREENSHOTS_DIR)/prq.tape
	vhs $(SCREENSHOTS_DIR)/ocm.tape
	rm -f $(PRQ_MOCK) $(OCM_MOCK)

$(PRQ_MOCK):
	go build -tags mock -o $(PRQ_MOCK) ./cmd/prq

$(OCM_MOCK):
	go build -tags mock -o $(OCM_MOCK) ./cmd/ocm

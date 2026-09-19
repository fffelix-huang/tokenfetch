GO       ?= go
REVISION := $(shell git rev-parse --short HEAD 2> /dev/null || echo devel)

.PHONY: build test lint clean

# Local build; --version shows the commit instead of "devel".
build:
	$(GO) build -trimpath -ldflags "-s -w -X main.revision=$(REVISION)" \
		-o bin/tokenfetch ./cmd/tokenfetch

test:
	$(GO) test ./...

lint:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	$(GO) vet ./...

clean:
	$(RM) -r bin

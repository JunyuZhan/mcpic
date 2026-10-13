BINARY := mcpic
PKG    := ./...

.PHONY: build test lint run clean

build:
	go build -o $(BINARY) ./cmd/mcpic

test:
	go test $(PKG)

lint:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal && exit 1)
	go vet $(PKG)

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

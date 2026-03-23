.PHONY: build test clean docker run

BINARY := resolver

build:
	go build -o $(BINARY) ./cmd/resolver

test:
	go test ./...

clean:
	rm -f $(BINARY)

docker:
	docker build -t settled-resolver .

run: build
	./$(BINARY)

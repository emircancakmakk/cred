.PHONY: build test

build:
	go build -o cred .

test:
	go test ./...

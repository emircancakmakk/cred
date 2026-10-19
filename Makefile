.PHONY: build test demo

build:
	go build -o cred .

test:
	go test ./...

demo:
	python3 demos/record.py

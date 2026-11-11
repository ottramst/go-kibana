default: fmt lint test

build:
	go build -v ./...

lint:
	golangci-lint run

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

# Needs a running Kibana: docker compose up -d --wait
testacc:
	KIBANA_ACC=1 go test -v -cover -timeout 15m ./...

.PHONY: default build lint fmt test testacc

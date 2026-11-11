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
	KIBANA_ACC=1 go test -v -cover -timeout 30m ./...

# Kibana OpenAPI specs that COVERAGE.md is generated from: the tested versions.
KIBANA_SPEC_VERSIONS ?= v8.19.25 v9.5.6
SPEC_DIR = .cache/specs

# Downloads the specs once into .cache/ and regenerates COVERAGE.md.
coverage:
	@mkdir -p $(SPEC_DIR)
	@for v in $(KIBANA_SPEC_VERSIONS); do \
		f=$(SPEC_DIR)/kibana-$$v.yaml; \
		[ -s $$f ] || curl -fsSL -o $$f https://raw.githubusercontent.com/elastic/kibana/$$v/oas_docs/output/kibana.yaml || exit 1; \
	done
	cd tools/coverage && go run . -root ../.. -versions "$(KIBANA_SPEC_VERSIONS)"

.PHONY: default build lint fmt test testacc coverage

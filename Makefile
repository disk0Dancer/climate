GOFLAGS ?=

.PHONY: build test fmt fmt-check lint vet verify

build:
	go build ./...

test:
	go test -race -coverprofile=coverage.out ./...

fmt:
	gofmt -w .

fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt-formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

lint:
	golangci-lint run

vet:
	go vet ./...

# Full pre-push gate: mirrors the CI pipeline. Run before every push.
verify: fmt-check vet build lint test

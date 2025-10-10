.PHONY: test fmt lint lint-fast
test:
	go test ./...

fmt:
	golangci-lint fmt --write ./...

lint:
	golangci-lint run ./...

lint-fast:
	golangci-lint run --new-from-rev=origin/main

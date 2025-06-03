# Makefile for DecTek Go project

.PHONY: tidy test lint fmt vet clean

# Run go mod tidy to clean up dependencies
tidy:
	go mod tidy

# Run all tests with coverage
test:
	go test -v -cover ./...

# Run staticcheck linter (requires staticcheck to be installed)
lint:
	golangci-lint run

# Run go fmt to format code
fmt:
	gofmt -s -w .

# Run go vet for static analysis
vet:
	go vet ./...

# Clean up test cache and build artifacts
clean:
	go clean -testcache

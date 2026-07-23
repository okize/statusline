BINARY := statusline

.PHONY: build test vet lint fmt clean

# Build the statusline binary. Point statusLine.command in settings.json at the
# resulting ./statusline.
build:
	go build -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

# Requires golangci-lint (https://golangci-lint.run/welcome/install/). CI runs
# the same linter, pinned in .github/workflows/ci.yml.
lint:
	golangci-lint run

fmt:
	gofmt -l -w .

clean:
	rm -f $(BINARY)

BINARY := statusline

.PHONY: build test coverage coverage-html vet lint fmt clean

# Build the statusline binary. Point statusLine.command in settings.json at the
# resulting ./statusline.
build:
	go build -o $(BINARY) .

test:
	go test ./...

# Per-function coverage table plus a total. -coverpkg=./... so tests in one
# package count toward coverage of the others.
coverage:
	go test -coverpkg=./... -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Same profile, rendered as an annotated HTML report in the browser.
coverage-html: coverage
	go tool cover -html=coverage.out

vet:
	go vet ./...

# Requires golangci-lint (https://golangci-lint.run/welcome/install/). CI runs
# the same linter, pinned in .github/workflows/ci.yml.
lint:
	golangci-lint run

fmt:
	gofmt -l -w .

clean:
	rm -f $(BINARY) coverage.out

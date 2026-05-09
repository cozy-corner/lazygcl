.PHONY: install
install: ## install pinned tools and pre-commit hooks
	mise install
	mise exec -- lefthook install

.PHONY: build
build: ## build the lazygcl binary
	mise exec -- go build -o lazygcl ./cmd/lazygcl

.PHONY: test
test: ## run unit tests
	mise exec -- go test ./...

.PHONY: lint
lint: ## run static analysis
	mise exec -- golangci-lint run ./...

.PHONY: fmt
fmt: ## format the codebase in place
	mise exec -- gofmt -w .
	mise exec -- goimports -w .

.PHONY: vuln
vuln: ## scan for known Go vulnerabilities
	mise exec -- govulncheck ./...

.PHONY: check
check: lint test vuln ## run everything CI runs

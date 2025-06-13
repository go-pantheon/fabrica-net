GOCMD=GO111MODULE=on CGO_ENABLED=0 go
GOBUILD=${GOCMD} build

# golangci-lint
LINTER := bin/golangci-lint
TOOLS_SHELL="./.hack/tools.sh"

$(LINTER):
	curl -SL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v2.1.1

.PHONY: init
# Initialize environment
init:
	pre-commit install
	go install github.com/git-chglog/git-chglog/cmd/git-chglog@latest

.PHONY: test
# Run tests
test:
	go test -v ./... -cover

.PHONY: vet
# Run go vet
vet:
	go vet ./...

.PHONY: lint
# Run lint
lint:
	golangci-lint run ./...

.PHONY: license-check
# Check licenses of dependencies
license-check:
	./.hack/licenses-check

.PHONY: license-report
# Generate license report
license-report:
	./.hack/licenses-generate
.PHONY: test-coverage

# Show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

COVERAGE_FILE ?= coverage.out

# Get all directories in cmd/ as available modules
MODULES := $(notdir $(wildcard cmd/*))

# Help target - display usage information
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  \033[36mmake build\033[0m - Build all modules ($(MODULES))"
	@$(foreach mod,$(MODULES),echo "  \033[36mmake build_$(mod)\033[0m - Build $(mod) module";)
	@echo "  \033[36mmake test\033[0m - Run all tests"

.PHONY: build
build:
	@echo "Building all modules: $(MODULES)"
	@mkdir -p bin
	@$(foreach mod,$(MODULES),echo "Building module: $(mod)"; go build -o ./bin/$(mod) ./cmd/$(mod);)

# Convenience targets for building individual modules
.PHONY: $(addprefix build_,$(MODULES))
$(addprefix build_,$(MODULES)):
	@modulename=$(subst build_,,$@); \
	echo "Building module: $$modulename"; \
	mkdir -p bin; \
	go build -o ./bin/$$modulename ./cmd/$$modulename

## test: run all tests
.PHONY: test
test:
	@go test -coverpkg='gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/...' --race -count=1 -coverprofile='$(COVERAGE_FILE)' ./...
	@go tool cover -func='$(COVERAGE_FILE)' | grep ^total | tr -s '\t'

.PHONY: build-scrapper build-bot build-images
build-scrapper:
	docker build -f Dockerfile.scrapper -t link-tracker-scrapper .
build-bot:
	docker build -f Dockerfile.bot -t link-tracker-bot .
build-agent:
	docker build -f Dockerfile.agent -t link-tracker-agent .
build-images: build-scrapper build-bot build-agent

.PHONY: test-http test-kafka
test-http:
	go test -v -count=1 -timeout 10m -run "TestBotScrapperSuite" ./tests/integration/...
test-kafka:
	go test -v -count=1 -timeout 10m -run "TestBotScrapperKafkaSuite" ./tests/integration/...

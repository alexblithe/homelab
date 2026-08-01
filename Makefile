.PHONY: help clean test test-go test-helm lint-go fmt-go vet-go

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  clean       Remove caches"
	@echo "  test        Run all tests (helm + go)"
	@echo "  test-go     Run Go integration tests"
	@echo "  test-helm   Run helm-unittest chart template tests"
	@echo "  lint-go     Run go vet on Go tests"
	@echo "  fmt-go      Format Go code"
	@echo "  vet-go      Run go vet"

clean:
	rm -rf __pycache__ .pytest_cache tests/__pycache__

test: test-helm test-go

test-go:
	go test ./tests/... -v

lint-go: vet-go

fmt-go:
	@gofmt -l -w tests/

vet-go:
	go vet ./tests/...

lint-helm:
	@for chart_dir in charts/*/; do \
		if [ -f "$$chart_dir/Chart.yaml" ]; then \
			echo "==> Linting $$(basename $$chart_dir)..."; \
			helm lint "$$chart_dir"; \
		fi; \
	done

test-helm:
	@for chart_dir in charts/*/; do \
		if [ -d "$$chart_dir/tests" ]; then \
			echo "==> Testing $$(basename $$chart_dir)..."; \
			helm unittest "$$chart_dir"; \
		fi; \
	done

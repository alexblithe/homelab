.PHONY: help venv install clean test test-unit test-helm

VENV_DIR := .venv

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  venv        Create Python virtual environment"
	@echo "  install     Install project dependencies into venv"
	@echo "  clean       Remove venv and all caches"
	@echo "  test        Run all tests (helm + pytest)"
	@echo "  test-unit   Run pytest tests"
	@echo "  test-helm   Run helm-unittest chart template tests"

venv:
	@if [ -d "$(VENV_DIR)" ]; then \
		echo "venv already exists at $(VENV_DIR)"; \
	else \
		python3 -m venv $(VENV_DIR); \
		echo "Created venv at $(VENV_DIR)"; \
	fi

install: venv
	@if [ ! -f "$(VENV_DIR)/bin/pip" ]; then \
		echo "Error: pip not found in $(VENV_DIR). Run 'make clean venv' to recreate."; \
		exit 1; \
	fi
	$(VENV_DIR)/bin/pip install -e .

clean:
	rm -rf $(VENV_DIR) __pycache__ .pytest_cache tests/__pycache__

test: test-helm test-unit

test-unit: install
	@if [ ! -f "$(VENV_DIR)/bin/pytest" ]; then \
		echo "Error: pytest not found. Run 'make install' first."; \
		exit 1; \
	fi
	$(VENV_DIR)/bin/pytest tests/ -v

test-helm:
	@for chart_dir in charts/*/; do \
		if [ -d "$$chart_dir/tests" ]; then \
			echo "==> Testing $$(basename $$chart_dir)..."; \
			helm unittest "$$chart_dir"; \
		fi; \
	done

ENV_NAME=chunker-v1-3.13.0
VENV := $(HOME)/.pyenv/versions/$(ENV_NAME)
PYTHON := $(VENV)/bin/python
PIP := $(VENV)/bin/pip

.PHONY: init-python
init-python:
	@if ! pyenv virtualenvs --bare | grep -qx "$(ENV_NAME)"; then \
		pyenv virtualenv 3.13.0 $(ENV_NAME); \
	fi

	$(PIP) install -r requirements.txt
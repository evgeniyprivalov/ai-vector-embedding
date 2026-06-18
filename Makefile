# PYTHON
ENV_NAME=chunker-v1-3.13.0
VENV := $(HOME)/.pyenv/versions/$(ENV_NAME)
PYTHON := $(VENV)/bin/python
PIP := $(VENV)/bin/pip


.PHONY: init-python
init-python:
	@if ! pyenv virtualenvs --bare | grep -qx "$(ENV_NAME)"; then \
		pyenv virtualenv 3.13.0 $(ENV_NAME); \
	fi

	$(PIP) install -r ./backend/chunker/requirements.txt


# GOLANG
.PHONY: migrations-local
migrations-local:
	set -a; . ./.env; set +a && goose -dir=./migrations -allow-missing up


# COMMON
.PHONY: generate-proto
generate-proto:
	@if [ ! -d "$(ENV_NAME)" ]; then \
		python3 -m venv $(ENV_NAME); \
	fi

	$(PYTHON) -m pip install --upgrade pip
	$(PYTHON) -m pip install grpcio grpcio-tools

	mkdir -p backend/core/internal/contracts/chunker
	mkdir -p backend/chunker/src/contracts/chunker

	protoc \
		-I backend/contracts/public/chunker backend/contracts/public/chunker/v1/*.proto \
		--go_out=./backend/core/internal/contracts/chunker \
		--go_opt=paths=source_relative \
		--go-grpc_out=./backend/core/internal/contracts/chunker \
		--go-grpc_opt=paths=source_relative

	$(PYTHON) -m grpc_tools.protoc \
		-I backend/contracts/public/chunker \
		--pyi_out=backend/chunker/src/contracts/chunker \
		--python_out=backend/chunker/src/contracts/chunker \
		--grpc_python_out=backend/chunker/src/contracts/chunker \
		backend/contracts/public/chunker/v1/*.proto

	sed -i '' 's/^from v1 import/from . import/' ./backend/chunker/src/contracts/chunker/v1/chunker_pb2_grpc.py


.PHONY: generate-openapi
generate-openapi:
	$(HOME)/go/bin/oapi-codegen --version

	# Validate the OpenAPI schema
	openapi-generator validate -i backend/contracts/public/openapi/v1/openapi.yaml

	# Generate intermediate YAML file
	openapi-generator generate -i backend/contracts/public/openapi/v1/openapi.yaml -g openapi-yaml -o backend/core/internal/contracts/public/v1/openapi
	mv backend/core/internal/contracts/public/v1/openapi/openapi/openapi.yaml backend/core/docs/public/openapi/v1/openapi.yaml
	rm -rf backend/core/internal/contracts/public/v1/openapi/.openapi-generator \
		backend/core/internal/contracts/public/v1/openapi/openapi \
		backend/core/internal/contracts/public/v1/openapi/.openapi-generator-ignore \
		backend/core/internal/contracts/public/v1/openapi/README.md

	$(HOME)/go/bin/oapi-codegen \
		-generate types,gorilla-server,strict-server \
		-templates backend/core/internal/config/openapi/templates/ \
		-package api \
		-o backend/core/internal/contracts/public/v1/openapi/openapi.gen.go \
		backend/core/docs/public/openapi/v1/openapi.yaml

	yq -p yaml -o json backend/core/docs/public/openapi/v1/openapi.yaml > backend/core/docs/public/openapi/v1/openapi.json

.PHONY: generate-openapi-sse
generate-openapi-sse:
	# Validate the OpenAPI schema
	openapi-generator validate -i backend/contracts/public/openapi/sse/v1/openapi.yaml

	# Generate intermediate YAML file
	openapi-generator generate -i backend/contracts/public/openapi/sse/v1/openapi.yaml -g openapi-yaml -o backend/core/internal/contracts/public/sse/v1/openapi
	mv backend/core/internal/contracts/public/sse/v1/openapi/openapi/openapi.yaml backend/core/docs/public/openapi/sse/v1/openapi.yaml
	rm -rf backend/core/internal/contracts/public/sse/v1/openapi/.openapi-generator \
		backend/core/internal/contracts/public/sse/v1/openapi/openapi \
		backend/core/internal/contracts/public/sse/v1/openapi/.openapi-generator-ignore \
		backend/core/internal/contracts/public/sse/v1/openapi/README.md

	oapi-codegen \
		-generate types,gorilla-server \
		-templates backend/core/internal/config/openapi/sse/templates/ \
		-package api \
		-o backend/core/internal/contracts/public/sse/v1/openapi/openapi.sse.gen.go \
		backend/core/docs/public/openapi/sse/v1/openapi.yaml

	yq -p yaml -o json backend/core/docs/public/openapi/sse/v1/openapi.yaml > backend/core/docs/public/openapi/v1/openapi.json


.PHONY: build-all
build-all:
	make init-python
	make generate-proto
	make generate-openapi


.PHONY: launch
launch:
	docker-compose up -d api-server --build
	sleep 5
	make migrations-local

# ai-vector-embedding

Polyglot vector search backend for document ingestion and semantic search.

## What this project is

This repository implements a small RAG-style backend:

- upload documents (`pdf`, `docx`, `md`, `txt`)
- split them into chunks
- generate embeddings
- store vectors in PostgreSQL with `pgvector`
- run semantic search by nearest vector distance

## Why there are 2 services in different languages

This project intentionally uses two backend services because they solve different problems best:

1. **Core API service (Go)**
   - Handles HTTP API, orchestration, DB access, search queries, and infrastructure concerns.
   - Go is a good fit for predictable performance, static typing, fast startup, and operational simplicity.

2. **Chunker service (Python)**
   - Handles document parsing and chunk extraction.
   - Python is a good fit because document/NLP tooling (LangChain loaders and splitters) is mature and easy to compose.

### Why this split is useful

- Keeps the API/data layer stable and production-oriented in Go.
- Keeps document parsing logic flexible in Python, where file format and text processing libraries evolve quickly.
- Allows each service to scale and deploy independently.
- Clear interface boundary through gRPC contract (`chunker.proto`) reduces coupling.

## High-level architecture

- **`backend/core` (Go)**: main API server and business logic.
- **`backend/chunker` (Python)**: document extraction and chunking service.
- **`backend/contracts/public`**: source contracts:
  - OpenAPI for HTTP API
  - Protobuf for gRPC chunking service
- **`postgresql`**: stores `documents(content, embedding)` with `VECTOR(768)` and HNSW cosine index.
- **`observability/`**: OpenTelemetry Collector + Tempo + Loki + Grafana.
- **`docker-compose.yaml`**: wires all services together.

## How the services work together

### 1) Document upload flow

1. Client sends `POST /v1/documents/upload` (multipart file) to Go API.
2. Go opens a streaming gRPC call to Python chunker (`ExtractChunks`).
3. Go streams raw file bytes; Python reconstructs and parses the file by extension.
4. Python splits text into chunks and streams chunk text back.
5. Go generates embedding for each chunk via configured AI endpoint.
6. Go writes chunk text + embedding vector into PostgreSQL.

### 2) Search flow

1. Client sends `POST /v1/search` with a text query.
2. Go embeds the query.
3. Go executes pgvector nearest-neighbor query (`embedding <=> query_vector`).
4. API returns best matches with similarity scores.

## Contracts and boundaries

- **HTTP contract**: OpenAPI under `backend/contracts/public/openapi/v1`.
- **Service-to-service contract**: Protobuf/gRPC under `backend/contracts/public/chunker/v1/chunker.proto`.
- Generated code in `backend/core/internal/contracts/...` and `backend/chunker/src/contracts/...` keeps implementations aligned.

## Runtime composition

With Docker Compose, the full stack typically includes:

- `api-server` (Go)
- `chunker` (Python)
- `postgresql` (pgvector)
- `otel-collector`, `tempo`, `loki`, `grafana`

The Go service is the entrypoint for clients. The Python service is internal and called by Go over gRPC.

## Summary

This architecture is polyglot on purpose: Go runs the core API/search pipeline, Python specializes in text extraction/chunking. The result is a cleaner separation of responsibilities, better language-tool fit, and easier evolution of each part.
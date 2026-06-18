FROM mirror.gcr.io/golang:1.26.4-alpine3.22 AS builder

WORKDIR /app

RUN apk add --no-cache \
    git \
    build-base \
    ca-certificates \
    tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /app/bin/server ./cmd/server/main.go


FROM mirror.gcr.io/alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/bin/server /app/server

RUN addgroup -S app && adduser -S app -G app
USER app

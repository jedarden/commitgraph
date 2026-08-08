# admin-alias-configmap-watcher — Watch admin-alias ConfigMap and auto-sync to Postgres
# Built by the commitgraph-build WorkflowTemplate with a REPO-ROOT kaniko context
# and --dockerfile=containers/admin-alias-configmap-watcher/Dockerfile

FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git (required for go.mod downloads)
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy watcher source
COPY cmd/watch-admin-alias-configmap/*.go ./cmd/watch-admin-alias-configmap/
COPY pkg/watcher/*.go ./pkg/watcher/

# Build watcher binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o watch-admin-alias-configmap ./cmd/watch-admin-alias-configmap

# Copy loader source
COPY cmd/load-admin-aliases/*.go ./cmd/load-admin-aliases/
COPY pkg/pg/*.go ./pkg/pg/

# Build loader binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o load-admin-aliases ./cmd/load-admin-aliases

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy both binaries from builder
COPY --from=builder /app/watch-admin-alias-configmap .
COPY --from=builder /app/load-admin-aliases .

# Set timezone
ENV TZ=UTC

# The watcher is the main entrypoint; it execs the loader internally
CMD ["./watch-admin-alias-configmap"]

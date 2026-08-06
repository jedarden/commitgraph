# Login Revalidation Worker — detect renamed/deleted GitHub logins
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o login-revalidation-worker .

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/login-revalidation-worker .

# Set timezone
ENV TZ=UTC

CMD ["./login-revalidation-worker"]

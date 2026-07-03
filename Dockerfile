# Stage 1: Build the Go binary
FROM golang:1.26-bookworm AS builder

WORKDIR /app

# Pre-copy/cache go.mod and go.sum to optimize layer caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compile static binary for maximum portability inside container
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o agent-runner ./cmd/agent-runner

# Stage 2: Final runner container with python3 and compiler dependencies for validation sandbox
FROM debian:bookworm-slim

# Install python3, build-essential (g++) for code sandbox validation
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    g++ \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/agent-runner .

# Run as non-root user for optimal shift-left container security
USER 10001:10001

ENTRYPOINT ["/app/agent-runner"]

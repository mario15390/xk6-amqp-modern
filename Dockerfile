# Dockerfile — Multi-stage build for k6 with xk6-amqp extension
#
# Stage 1 (builder): Compiles a custom k6 binary with the AMQP extension
#   using the xk6 tool. Uses Go 1.25 as required by latest k6.
#
# Stage 2 (runtime): Copies the compiled binary into a minimal Alpine image
#   for a small, production-ready container.
#
# Build:
#   docker build -t k6-amqp:local .
#
# Run:
#   docker run --rm k6-amqp:local run /scripts/test.js

# --- Stage 1: Build ---
FROM golang:1.25-alpine AS builder

# Install git (required by xk6 to resolve module dependencies)
RUN apk add --no-cache git

# Install the xk6 build tool
RUN go install go.k6.io/xk6/cmd/xk6@latest

# Copy the extension source code
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build k6 with our extension compiled in.
# The --with flag tells xk6 to include our local module as the extension.
RUN xk6 build \
    --with github.com/mario15390/xk6-amqp-modern=. \
    --output /k6

# --- Stage 2: Runtime ---
FROM alpine:3.20

# Install CA certificates (needed for AMQPS connections)
RUN apk add --no-cache ca-certificates

# Copy the compiled k6 binary from the builder stage
COPY --from=builder /k6 /usr/bin/k6

# Copy integration test scripts for easy access
COPY integration/scripts/ /scripts/

ENTRYPOINT ["k6"]

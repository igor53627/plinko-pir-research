# Multi-stage build for Piano PIR Server
# Stage 1: Build Go binary
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Copy Go module files
COPY go.mod ./
RUN go mod download

# Copy source code
COPY *.go ./

# Build binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" -o pir-server .

# Stage 2: Minimal runtime image
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates curl

# Copy binary from builder
COPY --from=builder /build/pir-server /app/pir-server

# Output directory
VOLUME /data

# Expose HTTP port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=3 \
  CMD curl -sf http://localhost:3000/health || exit 1

# Run the server
ENTRYPOINT ["/app/pir-server"]

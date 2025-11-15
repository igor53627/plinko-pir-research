# Multi-stage build for the state-syncer service
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Cache modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY *.go ./

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o state-syncer .

# Runtime image
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /build/state-syncer /app/state-syncer

VOLUME /data
VOLUME /public

EXPOSE 3002

ENTRYPOINT ["/app/state-syncer"]

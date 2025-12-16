FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go.mod files for dependency resolution
COPY bff/go.mod bff/go.sum ./bff/
COPY gen/go.mod gen/go.sum ./gen/
COPY pkg/connect/go.mod pkg/connect/go.sum ./pkg/connect/

# Download dependencies
WORKDIR /app/bff
RUN go mod download

# Copy source
WORKDIR /app
COPY bff/ ./bff/
COPY gen/ ./gen/
COPY pkg/connect/ ./pkg/connect/

# Build
WORKDIR /app/bff
RUN CGO_ENABLED=0 GOOS=linux go build -o /bff ./cmd/server

# Runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

COPY --from=builder /bff /bff

EXPOSE 8080

ENTRYPOINT ["/bff"]

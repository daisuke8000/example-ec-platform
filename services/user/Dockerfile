FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

# Copy go.mod files for dependency resolution
COPY services/user/go.mod services/user/go.sum ./services/user/
COPY gen/go.mod gen/go.sum ./gen/
COPY pkg/connect/go.mod pkg/connect/go.sum ./pkg/connect/

# Download dependencies
WORKDIR /app/services/user
RUN go mod download

# Copy source
WORKDIR /app
COPY services/user/ ./services/user/
COPY gen/ ./gen/
COPY pkg/connect/ ./pkg/connect/

# Build
WORKDIR /app/services/user
RUN CGO_ENABLED=0 GOOS=linux go build -o /user-service ./cmd/server

# Runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

COPY --from=builder /user-service /user-service

EXPOSE 50051 8051

ENTRYPOINT ["/user-service"]

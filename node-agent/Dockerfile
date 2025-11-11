FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install SQLite dependencies
RUN apk add --no-cache sqlite-dev gcc musl-dev

# Copy go mod files
COPY go.mod ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/agent ./cmd

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite

WORKDIR /app

COPY --from=builder /app/agent .

# Create data directory
RUN mkdir -p /var/lib/node-agent

EXPOSE 8080

CMD ["./agent"]


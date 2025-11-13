FROM golang:1.23-alpine AS builder

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

# Create directories and copy configuration file
RUN mkdir -p /app/conf /var/lib/node-agent
COPY conf/config.yaml /app/conf/config.yaml

EXPOSE 8080

CMD ["./agent"]


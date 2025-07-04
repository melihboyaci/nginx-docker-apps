# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files (handle missing go.sum)
COPY go.mod ./
COPY go.su[m] ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o main .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .
COPY --from=builder /app/index.html .

EXPOSE 8080
EXPOSE 80
EXPOSE 443

CMD ["./main"]
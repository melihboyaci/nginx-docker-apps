FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Clean go modules cache and download dependencies
RUN go clean -modcache
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Create uploads directory
RUN mkdir -p uploads

# Copy static files if they exist
COPY static ./static/ 2>/dev/null || true
COPY *.html ./ 2>/dev/null || true

EXPOSE 80 443

CMD ["./main"]

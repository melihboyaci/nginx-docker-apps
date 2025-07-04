# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./

RUN go clean -modcache
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/main .

RUN mkdir -p uploads

COPY static ./static/ 2>/dev/null || true
COPY *.html ./ 2>/dev/null || true

EXPOSE 80 443

CMD ["./main"]
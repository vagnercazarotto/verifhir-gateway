# ---- build stage -----------------------------------------------------------
FROM golang:1.25-rc-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=1 is required by modernc.org/sqlite (pure-Go, but uses cgo stubs)
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/gateway

# ---- runtime stage ---------------------------------------------------------
FROM alpine:3.20
WORKDIR /app

COPY --from=builder /app/gateway .
COPY configs/ configs/

EXPOSE 8080
CMD ["./gateway"]

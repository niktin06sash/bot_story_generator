FROM golang:1.25.2 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o service ./cmd/app/main.go

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/service .

CMD ["./service"]
# ---- Build Stage ----
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o splitwise-server cmd/server/main.go

# ---- Run Stage ----
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /app/splitwise-server .

EXPOSE 8080

CMD ["./splitwise-server"]

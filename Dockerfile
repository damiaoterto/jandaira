FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git build-base

COPY . .

RUN go mod download
RUN CGO_ENABLED=1 GOOS=linux go build -o jandaira-api ./cmd/api/main.go

FROM alpine:latest

RUN adduser -D appuser
USER appuser

WORKDIR /app

COPY --from=builder /app/jandaira-api /app/jandaira

EXPOSE 8080

CMD ["sh", "-c", "/app/jandaira"]
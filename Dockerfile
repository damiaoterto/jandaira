FROM golang:1.26-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    git build-essential \
    && rm -rf /var/lib/apt/lists/*

COPY . .

RUN go mod download
RUN CGO_ENABLED=1 GOOS=linux go build -o jandaira-api ./cmd/api/main.go

FROM golang:1.26-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libstdc++6 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -r -m -d /home/appuser -s /bin/false appuser

WORKDIR /app

COPY --from=builder /app/jandaira-api .

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

CMD ["sh", "-c", "/app/jandaira-api"]
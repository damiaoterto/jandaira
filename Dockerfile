FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache build-base ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-w -s -extldflags '-static'" \
    -trimpath \
    -o jandaira-api ./cmd/api/main.go

# pre-create data dir owned by runtime user
RUN mkdir -p /home/nonroot/.config/jandaira && \
    chown -R 1001:1001 /home/nonroot && \
    echo "nonroot:x:1001:1001::/home/nonroot:/sbin/nologin" > /etc_passwd

FROM scratch

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc_passwd /etc/passwd
COPY --from=builder /home/nonroot /home/nonroot
COPY --from=builder /app/jandaira-api /app/jandaira

USER 1001

EXPOSE 8080

CMD ["/app/jandaira"]

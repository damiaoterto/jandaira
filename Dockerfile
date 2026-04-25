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

FROM nginx:alpine

WORKDIR /app

ARG FRONTEND_VERSION="1.0.2"

RUN apk add --no-cache curl bash ca-certificates \
     openssl ncurses coreutils make gcc g++ libgcc \
    linux-headers caddy

COPY entrypoint.sh ./
RUN chmod +x entrypoint.sh

COPY --from=builder /app/jandaira-api /app/jandaira
ADD ./jandaira-frontend-v${FRONTEND_VERSION}.tgz /usr/share/nginx/html/
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 9000 8080

CMD ["sh", "-c", "/app/entrypoint.sh"]

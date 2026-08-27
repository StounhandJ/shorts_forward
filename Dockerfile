FROM golang:1.26.7-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-s -w" -o /app/shorts_forward cmd/app/main.go

FROM alpine:3.21

ARG YT_DLP_VERSION=2026.08.19

ENV TZ=Europe/Moscow

# Установка зависимостей (python3 обязателен для yt-dlp, ffmpeg для обработки видео)
RUN apk add --upgrade --no-cache \
    tzdata \
    python3 \
    ca-certificates \
    curl \
    ffmpeg \
    && cp /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone \
    && curl -L https://github.com/yt-dlp/yt-dlp/releases/download/${YT_DLP_VERSION}/yt-dlp -o /usr/local/bin/yt-dlp \
    && chmod a+rx /usr/local/bin/yt-dlp

COPY --from=builder /app/shorts_forward ./shorts_forward
COPY config ./config

ENTRYPOINT ["./shorts_forward"]
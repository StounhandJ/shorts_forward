FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-s -w" -o /app/shorts_forward cmd/app/main.go

FROM alpine:3.21
ENV TZ=Europe/Moscow
RUN apk add --upgrade --no-cache tzdata
RUN cp /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

COPY --from=builder /app/shorts_forward ./shorts_forward
COPY config ./config

ENTRYPOINT ["./shorts_forward"]
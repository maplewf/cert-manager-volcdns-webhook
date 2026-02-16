FROM golang:1.24.5-alpine AS builder

WORKDIR /workspace

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o cert-manager-volcdns-webhook .

FROM alpine:latest

RUN addgroup -g 1000 webhook && \
    adduser -D -u 1000 -G webhook webhook

WORKDIR /app

RUN chown -R webhook:webhook /app

COPY --from=builder /workspace/cert-manager-volcdns-webhook /app/cert-manager-volcdns-webhook

USER 1000

ENTRYPOINT ["/app/cert-manager-volcdns-webhook"]


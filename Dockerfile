# syntax=docker/dockerfile:1
# 多阶段构建：编译静态二进制（纯 Go，无 CGO），运行时仅需 CA 证书。
FROM golang:1.26-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/mcpic ./cmd/mcpic

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 mcpic \
    && mkdir -p /data && chown mcpic:mcpic /data
COPY --from=build /out/mcpic /usr/local/bin/mcpic
ENV MCPIC_STORAGE_PATH=/data
VOLUME /data
EXPOSE 8080
USER mcpic
ENTRYPOINT ["mcpic"]

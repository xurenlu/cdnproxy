# syntax=docker/dockerfile:1

# --- build stage ---
FROM golang:1.22-alpine AS build
WORKDIR /app
# 安装WebP开发库和构建工具
RUN apk add --no-cache git ca-certificates libwebp-dev gcc musl-dev && update-ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 启用CGO以支持WebP库
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /bin/cdnproxy ./

# --- run stage ---
FROM alpine:latest
WORKDIR /
# 安装运行时所需的WebP库
RUN apk add --no-cache ca-certificates libwebp
COPY --from=build /bin/cdnproxy /cdnproxy

# 环境变量
ENV PORT=8080
ENV WEBP_ENABLED=false

EXPOSE 8080
ENTRYPOINT ["/cdnproxy"]

# 多阶段构建 Dockerfile for x64 架构
# 构建阶段
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git make

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用程序
RUN make build-linux && cp bin/linux-amd64/llmproxyd /usr/local/bin/llmproxyd

# 运行阶段
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 设置时区为中国上海
ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# 创建非特权用户
RUN addgroup -g 1001 -S llmproxy && \
    adduser -u 1001 -S llmproxy -G llmproxy

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /usr/local/bin/llmproxyd /app/llmproxyd

# 更改文件所有者
RUN chown -R llmproxy:llmproxy /app

# 切换到非特权用户
USER llmproxy

# 设置入口点
ENTRYPOINT ["/app/llmproxyd"]
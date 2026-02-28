# 使用Go官方镜像
FROM golang:1.25 AS builder

# Install gcc and libc-dev for sqlite3 (CGO requirement)
RUN apt-get update && apt-get install -y gcc libc-dev

# 设置工作目录
WORKDIR /app

# 将本地代码复制到容器中
COPY . .

# 下载 Go 依赖并编译
ARG APP_VERSION=unknown
RUN go mod tidy
RUN go build -ldflags "-X main.Version=${APP_VERSION}" -o ai-platform .

# Stage 2: Runner
# Use the same OS base (bookworm) to ensure Glibc compatibility
FROM debian:bookworm-slim

# 安装必要的依赖
RUN apt-get update && apt-get install -y ca-certificates && apt-get clean

# 将编译好的可执行文件复制到最终镜像中
COPY --from=builder /app/ai-platform /usr/local/bin/ai-platform

# 设置容器启动时执行的命令
ENTRYPOINT ["/usr/local/bin/ai-platform"]

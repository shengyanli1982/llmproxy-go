# LLMProxy

高性能 LLM API 代理服务器，为大语言模型 API 提供智能负载均衡、熔断保护与实时监控能力。

## 核心特性

-   **高性能代理** - 基于 Orbit 框架和连接池优化，支持 OpenAI、Anthropic 等主流 LLM API
-   **智能负载均衡** - 支持轮询、加权轮询、随机和 IP 哈希策略
-   **熔断保护** - 集成重试功能的智能熔断器，自动故障转移
-   **限流控制** - IP 级别和上游级别双重限流保护
-   **实时监控** - Prometheus 指标采集，提供健康检查接口
-   **灵活认证** - 支持 Bearer Token、Basic Auth 等多种认证方式
-   **HTTP 头操作** - 支持请求头的插入、替换和删除操作
-   **优雅关闭** - 支持信号处理和资源清理

## 快速开始

### 安装

```bash
# 下载源码
git clone https://github.com/shengyanli1982/llmproxy-go.git
cd llmproxy-go

# 编译
make build

# 或使用 Go 直接编译
go build -o llmproxy ./cmd/llmproxy
```

### 使用二进制

项目提供了预编译的二进制文件，您可以根据实际需求下载对应平台的二进制文件直接使用。

### 配置

复制配置模板并修改：

```bash
cp config.default.yaml config.yaml
# 编辑 config.yaml，配置上游服务和 API 密钥
# config.default.yaml 是参考配置文件，请根据实际情况手动配置
```

### 运行

```bash
# 开发模式
./llmproxy -c config.yaml

# 生产模式
./llmproxy -c config.yaml --release --json
```

## 配置说明

### HTTP 服务器配置

```yaml
httpServer:
    forwards:
        - name: to_mixgroup
          port: 3000 # 监听端口
          address: "0.0.0.0" # 监听地址
          defaultGroup: "mixgroup" # 默认上游组
          ratelimit: # IP 限流
              perSecond: 100
              burst: 200
          timeout: # 连接超时
              idle: 60000
              read: 30000
              write: 30000
    admin: # 管理接口
        port: 9000
        address: "0.0.0.0"
```

### 上游服务配置

```yaml
upstreams:
    - name: openai_primary
      url: "https://api.openai.com/v1/chat/completions"
      auth:
          type: "bearer"
          token: "YOUR_OPENAI_API_KEY_HERE"
      breaker: # 熔断器配置
          threshold: 0.5 # 失败率阈值
          cooldown: 30000 # 冷却时间(ms)
          maxRequests: 3 # 半开状态最大请求数
      ratelimit: # 上游限流
          perSecond: 100
```

### 上游组配置

```yaml
upstreamGroups:
    - name: mixgroup
      upstreams:
          - name: openai_primary
            weight: 8 # 权重(仅weighted_roundrobin)
      balance:
          strategy: "roundrobin" # 负载均衡策略
      httpClient:
          keepalive: 60000 # TCP Keepalive
          timeout:
              connect: 10000 # 连接超时
              request: 300000 # 请求超时
              idle: 60000 # 空闲超时
```

## 负载均衡策略

-   `roundrobin` - 轮询分配
-   `weighted_roundrobin` - 加权轮询
-   `random` - 随机选择（默认）
-   `iphash` - IP 哈希

## 监控端点

-   `GET /ping` - 健康检查
-   `GET /metrics` - Prometheus 指标

## Docker 部署

项目为 x64 平台提供了 Dockerfile，arm64 平台可使用 Dockerfile-arm64 构建。

```bash
# 构建镜像
docker build -t llmproxy .

# 运行容器
docker run -d \
  -p 3000:3000 \
  -p 9000:9000 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  llmproxy
```

## 命令行选项

```bash
llmproxy [flags]

Flags:
  -c, --config string   配置文件路径 (default "./config.yaml")
  -j, --json           启用 JSON 格式日志输出
  -r, --release        启用生产模式
  -h, --help           显示帮助信息
  -v, --version        显示版本信息
```

## 开发

### 构建

```bash
make build          # 编译
make build-linux    # Linux 交叉编译
make build-docker   # Docker 镜像构建
```

### 测试

```bash
make test           # 运行测试
make test-coverage  # 测试覆盖率
```

## 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 作者

[shengyanli1982](https://github.com/shengyanli1982)

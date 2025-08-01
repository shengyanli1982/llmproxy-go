# LLMProxy

高性能 LLM API 代理服务器，为大语言模型 API 提供智能负载均衡(支持账号均衡、请求熔断保护、限速、自定义修改请求头部)。

## 1. 核心特性

-   **高性能代理** - 基于 Orbit 框架和连接池优化，支持 OpenAI、Anthropic 等主流 LLM API
-   **智能负载均衡** - 支持轮询、加权轮询、随机和 IP 哈希策略
-   **熔断保护** - 集成重试功能的智能熔断器，自动故障转移
-   **限流控制** - IP 级别和上游级别双重限流保护
-   **实时监控** - Prometheus 指标采集，提供健康检查接口
-   **灵活认证** - 支持 Bearer Token、Basic Auth 等多种认证方式
-   **HTTP 头操作** - 支持请求头的插入、替换和删除操作
-   **优雅关闭** - 支持信号处理和资源清理

## 2. 能力详解

### HTTP 客户端优化

LLMProxy 提供企业级的 HTTP 客户端优化，解决大规模 LLM API 调用中的性能瓶颈：

-   **连接池管理** - 智能复用 TCP 连接，减少连接建立开销，支持按主机配置最大连接数和空闲连接数
-   **TCP Keepalive** - 保持长连接活跃状态，降低网络延迟，可配置 keepalive 时间
-   **代理支持** - 支持 HTTP/HTTPS 代理，适应企业网络环境和安全策略
-   **超时控制** - 精细化超时管理（连接、请求、空闲），防止资源泄露和请求阻塞

### 智能熔断保护

内置 Circuit Breaker 模式，自动处理上游服务故障，确保系统稳定性：

-   **失败率检测** - 实时监控上游服务失败率，可配置触发阈值（0.01-1.0）
-   **三态管理** - 闭合 → 开启 → 半开状态自动切换，智能故障恢复
-   **半开重试** - 在半开状态下限量测试上游恢复情况，避免雪崩效应
-   **冷却机制** - 可配置冷却时间，给上游服务恢复时间
-   **统计周期** - 定期重置失败统计，防止历史故障长期影响

### 多策略负载均衡

提供 4 种负载均衡策略，满足不同业务场景的流量分发需求：

-   **轮询(roundrobin)** - 平均分配请求，适用于同质化上游服务
-   **加权轮询(weighted_roundrobin)** - 按权重比例分配，适用于异构上游或成本优化
-   **随机(random)** - 随机选择上游，减少"热点"问题
-   **IP 哈希(iphash)** - 基于客户端 IP 的一致性路由，保持会话亲和性

负载均衡器从可用上游列表中选择目标服务，配合熔断器提供故障保护。

## 3. 快速开始

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

## 4. 快速配置

### 最小可启动配置

```yaml
httpServer:
    forwards:
        - name: simple_forward
          port: 3000
          defaultGroup: simple_group

upstreams:
    - name: openai_api
      url: "https://api.openai.com/v1/chat/completions"
      auth:
          type: bearer
          token: "YOUR_API_KEY_HERE"

upstreamGroups:
    - name: simple_group
      upstreams:
          - name: openai_api
```

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

## 5. 配置参数详解

### 主配置项

| 配置项           | 类型   | 必填 | 默认值 | 描述            |
| ---------------- | ------ | ---- | ------ | --------------- |
| `httpServer`     | object | ✓    | -      | HTTP 服务器配置 |
| `upstreams`      | array  | ✓    | -      | 上游服务列表    |
| `upstreamGroups` | array  | ✓    | -      | 上游组列表      |

### HTTP 服务器配置

| 配置项                                      | 类型   | 必填 | 默认值    | 描述                 |
| ------------------------------------------- | ------ | ---- | --------- | -------------------- |
| `httpServer.forwards`                       | array  | ✓    | -         | 转发服务列表         |
| `httpServer.forwards[].name`                | string | ✓    | -         | 转发服务名称         |
| `httpServer.forwards[].port`                | int    | ✓    | -         | 监听端口(1-65535)    |
| `httpServer.forwards[].address`             | string | -    | "0.0.0.0" | 监听地址             |
| `httpServer.forwards[].defaultGroup`        | string | ✓    | -         | 默认上游组名称       |
| `httpServer.forwards[].ratelimit.perSecond` | int    | -    | 100       | 每秒请求数限制       |
| `httpServer.forwards[].ratelimit.burst`     | int    | -    | 200       | 突发请求数限制       |
| `httpServer.forwards[].timeout.idle`        | int    | -    | 60000     | 空闲超时(ms)         |
| `httpServer.forwards[].timeout.read`        | int    | -    | 30000     | 读取超时(ms)         |
| `httpServer.forwards[].timeout.write`       | int    | -    | 30000     | 写入超时(ms)         |
| `httpServer.admin.port`                     | int    | -    | 9000      | 管理端口             |
| `httpServer.admin.address`                  | string | -    | "0.0.0.0" | 管理地址             |
| `httpServer.admin.timeout.idle`             | int    | -    | 60000     | 管理接口空闲超时(ms) |
| `httpServer.admin.timeout.read`             | int    | -    | 30000     | 管理接口读取超时(ms) |
| `httpServer.admin.timeout.write`            | int    | -    | 30000     | 管理接口写入超时(ms) |

### 上游服务配置

| 配置项                            | 类型   | 必填 | 默认值 | 描述                                   |
| --------------------------------- | ------ | ---- | ------ | -------------------------------------- |
| `upstreams[].name`                | string | ✓    | -      | 上游服务名称                           |
| `upstreams[].url`                 | string | ✓    | -      | 上游服务 URL                           |
| `upstreams[].auth.type`           | string | -    | "none" | 认证类型(none/bearer/basic)            |
| `upstreams[].auth.token`          | string | -    | -      | Bearer Token                           |
| `upstreams[].auth.username`       | string | -    | -      | Basic 认证用户名                       |
| `upstreams[].auth.password`       | string | -    | -      | Basic 认证密码                         |
| `upstreams[].headers[].op`        | string | -    | -      | HTTP 头操作类型(insert/replace/remove) |
| `upstreams[].headers[].key`       | string | -    | -      | HTTP 头名称                            |
| `upstreams[].headers[].value`     | string | -    | -      | HTTP 头值(remove 操作可省略)           |
| `upstreams[].breaker.threshold`   | float  | -    | 0.5    | 熔断失败率阈值(0.01-1.0)               |
| `upstreams[].breaker.cooldown`    | int    | -    | 30000  | 熔断冷却时间(ms)                       |
| `upstreams[].breaker.maxRequests` | int    | -    | 3      | 半开状态最大请求数                     |
| `upstreams[].breaker.interval`    | int    | -    | 10000  | 统计周期重置间隔(ms)                   |
| `upstreams[].ratelimit.perSecond` | int    | -    | 100    | 每秒请求数限制                         |

### 上游组配置

| 配置项                                            | 类型   | 必填 | 默认值         | 描述                         |
| ------------------------------------------------- | ------ | ---- | -------------- | ---------------------------- |
| `upstreamGroups[].name`                           | string | ✓    | -              | 上游组名称                   |
| `upstreamGroups[].upstreams`                      | array  | ✓    | -              | 上游服务引用列表             |
| `upstreamGroups[].upstreams[].name`               | string | ✓    | -              | 引用的上游服务名称           |
| `upstreamGroups[].upstreams[].weight`             | int    | -    | 1              | 权重(仅 weighted_roundrobin) |
| `upstreamGroups[].balance.strategy`               | string | -    | "roundrobin"   | 负载均衡策略                 |
| `upstreamGroups[].httpClient.agent`               | string | -    | "LLMProxy/1.0" | User-Agent                   |
| `upstreamGroups[].httpClient.keepalive`           | int    | -    | 60000          | TCP Keepalive(ms)            |
| `upstreamGroups[].httpClient.connect.idleTotal`   | int    | -    | 100            | 最大空闲连接数               |
| `upstreamGroups[].httpClient.connect.idlePerHost` | int    | -    | 10             | 每主机最大空闲连接数         |
| `upstreamGroups[].httpClient.connect.maxPerHost`  | int    | -    | 50             | 每主机最大连接数             |
| `upstreamGroups[].httpClient.timeout.connect`     | int    | -    | 10000          | 连接超时(ms)                 |
| `upstreamGroups[].httpClient.timeout.request`     | int    | -    | 300000         | 请求超时(ms)                 |
| `upstreamGroups[].httpClient.timeout.idle`        | int    | -    | 60000          | 空闲连接超时(ms)             |
| `upstreamGroups[].httpClient.proxy.url`           | string | -    | -              | HTTP/HTTPS 代理服务器 URL    |

## 6. 运维监控端点

-   `GET /ping` - 健康检查
-   `GET /metrics` - Prometheus 指标

## 7. Docker 部署

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

## 8. 命令行选项

```bash
llmproxy [flags]

Flags:
  -c, --config string   配置文件路径 (default "./config.yaml")
  -j, --json           启用 JSON 格式日志输出
  -r, --release        启用生产模式
  -h, --help           显示帮助信息
  -v, --version        显示版本信息
```

## 9. 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 10. 开发参与者

[shengyanli1982](https://github.com/shengyanli1982)

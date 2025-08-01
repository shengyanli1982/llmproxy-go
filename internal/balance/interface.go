package balance

import (
	"context"
	"errors"
	"net/http"

	"github.com/shengyanli1982/llmproxy-go/internal/auth"
	"github.com/shengyanli1982/llmproxy-go/internal/breaker"
	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/shengyanli1982/llmproxy-go/internal/ratelimit"
	"github.com/sony/gobreaker"
)

// clientIPKey 是用于在 context 中存储客户端 IP 的私有类型
// 使用私有类型作为 key 可以避免与其他包的 context key 冲突
type clientIPKey struct{}

// clientIPContextKey 是客户端 IP 的 context key 实例
var clientIPContextKey = clientIPKey{}

// 负载均衡相关错误定义
var (
	ErrNoAvailableUpstream = errors.New("no available upstream")
	ErrUnknownStrategy     = errors.New("unknown load balance strategy")
	ErrNilUpstreams        = errors.New("upstreams cannot be nil")
	ErrEmptyUpstreams      = errors.New("upstreams cannot be empty")
)

// Upstream 代表一个上游服务实例
type Upstream struct {
	Name   string                 // 上游服务名称
	URL    string                 // 上游服务 URL
	Weight int                    // 权重（用于加权轮询）
	Config *config.UpstreamConfig // 上游服务配置
	
	// 预初始化的组件实例，避免重复创建
	Authenticator auth.Authenticator       // 认证器（缓存）
	Breaker       breaker.CircuitBreaker   // 熔断器
	RateLimiter   *ratelimit.UpstreamLimiter // 限流器
}

// ApplyAuth 应用认证到HTTP请求
// 如果认证器未初始化，则跳过认证（默认行为）
func (u *Upstream) ApplyAuth(req *http.Request) error {
	if u.Authenticator != nil {
		return u.Authenticator.Apply(req)
	}
	return nil
}

// CheckRateLimit 检查是否通过限流检查
// 如果限流器未初始化，则默认允许通过
func (u *Upstream) CheckRateLimit() bool {
	if u.RateLimiter != nil {
		return u.RateLimiter.Allow(u.Name)
	}
	return true
}

// ExecuteWithBreaker 通过熔断器执行HTTP请求
// 如果熔断器未初始化，则直接执行请求函数
func (u *Upstream) ExecuteWithBreaker(fn func() (*http.Response, error)) (*http.Response, error) {
	if u.Breaker != nil {
		result, err := u.Breaker.Execute(func() (interface{}, error) {
			return fn()
		})
		if err != nil {
			return nil, err
		}
		return result.(*http.Response), nil
	}
	return fn()
}

// LoadBalancer 代表负载均衡器接口，定义选择上游服务的行为
type LoadBalancer interface {
	// Select 根据负载均衡策略选择一个上游服务
	// ctx: 上下文信息
	// upstreams: 可用的上游服务列表
	Select(ctx context.Context, upstreams []Upstream) (Upstream, error)

	// UpdateHealth 更新上游服务的健康状态
	// upstreamName: 上游服务名称
	// healthy: 健康状态
	UpdateHealth(upstreamName string, healthy bool)

	// UpdateLatency 更新上游服务的响应延迟（用于响应时间感知负载均衡）
	// upstreamName: 上游服务名称
	// latency: 响应延迟（毫秒）
	UpdateLatency(upstreamName string, latency int64)

	// Type 获取负载均衡器类型
	Type() string
}

// LoadBalancerWithBreaker 扩展负载均衡器接口，支持熔断器功能
type LoadBalancerWithBreaker interface {
	LoadBalancer

	// CreateBreaker 为上游服务创建熔断器
	CreateBreaker(upstreamName string, settings gobreaker.Settings) error

	// GetBreaker 获取指定上游的熔断器
	GetBreaker(upstreamName string) (breaker.CircuitBreaker, bool)
}

// LoadBalancerFactory 代表负载均衡器工厂接口
type LoadBalancerFactory interface {
	// Create 根据配置创建负载均衡器
	// config: 负载均衡配置
	Create(config *config.BalanceConfig) (LoadBalancer, error)
}

// WithClientIP 将客户端 IP 地址存储到 context 中
// 此函数用于在负载均衡器选择过程中传递客户端 IP 信息
// 主要用于基于客户端 IP 的负载均衡算法（如一致性哈希）
//
// 参数：
//   - ctx: 父级 context
//   - clientIP: 客户端 IP 地址字符串
//
// 返回值：
//   - 包含客户端 IP 信息的新 context
func WithClientIP(ctx context.Context, clientIP string) context.Context {
	return context.WithValue(ctx, clientIPContextKey, clientIP)
}

// GetClientIP 从 context 中获取客户端 IP 地址
// 此函数用于在负载均衡器中提取客户端 IP 信息
//
// 参数：
//   - ctx: 包含客户端 IP 信息的 context
//
// 返回值：
//   - clientIP: 客户端 IP 地址字符串，如果不存在则返回空字符串
//   - ok: 布尔值，表示是否成功获取到客户端 IP
func GetClientIP(ctx context.Context) (clientIP string, ok bool) {
	value := ctx.Value(clientIPContextKey)
	if value == nil {
		return "", false
	}

	clientIP, ok = value.(string)
	return clientIP, ok
}

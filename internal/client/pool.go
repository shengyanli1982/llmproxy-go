package client

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
)

// ConnectionPool 连接池管理器
type ConnectionPool struct {
	transport *http.Transport
	config    *config.HTTPClientConfig
}

// NewConnectionPool 创建新的连接池实例
func NewConnectionPool(cfg *config.HTTPClientConfig) *ConnectionPool {
	// 创建自定义Transport
	transport := &http.Transport{
		// TLS配置
		TLSHandshakeTimeout: 30 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},

		// Keep-Alive配置 - 如果KeepAlive为0，禁用Keep-Alive
		DisableKeepAlives: cfg.KeepAlive == 0,

		// 拨号配置
		DialContext: (&net.Dialer{
			KeepAlive: time.Duration(cfg.KeepAlive) * time.Second,
		}).DialContext,

		// 期望继续超时
		ExpectContinueTimeout: 1 * time.Second,
	}

	// 设置连接池配置
	if cfg.Connect != nil {
		transport.MaxIdleConns = cfg.Connect.IdleTotal
		transport.MaxIdleConnsPerHost = cfg.Connect.IdlePerHost
		transport.MaxConnsPerHost = cfg.Connect.MaxPerHost
	}

	// 设置超时配置
	if cfg.Timeout != nil {
		if cfg.Timeout.Connect > 0 {
			transport.DialContext = (&net.Dialer{
				Timeout:   time.Duration(cfg.Timeout.Connect) * time.Millisecond,
				KeepAlive: time.Duration(cfg.KeepAlive) * time.Millisecond,
			}).DialContext
		}
		if cfg.Timeout.Request > 0 {
			transport.ResponseHeaderTimeout = time.Duration(cfg.Timeout.Request) * time.Millisecond
		}
		if cfg.Timeout.Idle > 0 {
			transport.IdleConnTimeout = time.Duration(cfg.Timeout.Idle) * time.Millisecond
		}
	}

	// 配置代理
	if cfg.Proxy != nil {
		if proxyURL, err := url.Parse(cfg.Proxy.URL); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &ConnectionPool{
		transport: transport,
		config:    cfg,
	}
}

// GetTransport 获取HTTP传输层
func (p *ConnectionPool) GetTransport() *http.Transport {
	return p.transport
}

// Close 关闭连接池
func (p *ConnectionPool) Close() error {
	p.transport.CloseIdleConnections()
	return nil
}

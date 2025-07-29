package client

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"
)

// ConnectionPool 连接池管理器
type ConnectionPool struct {
	transport *http.Transport
	config    *Config
}

// NewConnectionPool 创建新的连接池实例
func NewConnectionPool(config *Config) *ConnectionPool {
	// 创建自定义Transport
	transport := &http.Transport{
		// 连接配置
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     config.MaxConnsPerHost,
		IdleConnTimeout:     time.Duration(config.IdleConnTimeout) * time.Second,

		// 拨号配置
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(config.ConnectTimeout) * time.Second,
			KeepAlive: 90 * time.Second,
		}).DialContext,

		// TLS配置
		TLSHandshakeTimeout: 30 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},

		// Keep-Alive配置
		DisableKeepAlives: !config.EnableKeepAlive,

		// 响应头超时
		ResponseHeaderTimeout: time.Duration(config.RequestTimeout) * time.Second,

		// 期望继续超时
		ExpectContinueTimeout: 1 * time.Second,
	}

	// 配置代理
	if config.ProxyURL != "" {
		if proxyURL, err := url.Parse(config.ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &ConnectionPool{
		transport: transport,
		config:    config,
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

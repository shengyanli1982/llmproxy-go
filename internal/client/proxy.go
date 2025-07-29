package client

import (
	"net/http"
	"net/url"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
)

// ProxyHandler 代理处理器
type ProxyHandler struct {
	proxyURL *url.URL
	enabled  bool
}

// NewProxyHandler 创建新的代理处理器实例
// 优先使用 proxyConfig 中的配置，如果没有设置则使用环境变量
func NewProxyHandler(proxyConfig *config.ProxyConfig) *ProxyHandler {
	// 如果没有提供代理配置或URL为空，则不启用代理
	if proxyConfig == nil || proxyConfig.URL == "" {
		return &ProxyHandler{
			enabled: false,
		}
	}

	parsedURL, err := url.Parse(proxyConfig.URL)
	if err != nil {
		return &ProxyHandler{
			enabled: false,
		}
	}

	return &ProxyHandler{
		proxyURL: parsedURL,
		enabled:  true,
	}
}

// NewProxyHandlerFromURL 从URL字符串创建代理处理器实例（保持向后兼容）
func NewProxyHandlerFromURL(proxyURL string) *ProxyHandler {
	if proxyURL == "" {
		return &ProxyHandler{
			enabled: false,
		}
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return &ProxyHandler{
			enabled: false,
		}
	}

	return &ProxyHandler{
		proxyURL: parsedURL,
		enabled:  true,
	}
}

// GetProxyFunc 获取代理函数
// 优先使用配置的代理，如果没有配置则使用环境变量
func (p *ProxyHandler) GetProxyFunc() func(*http.Request) (*url.URL, error) {
	if !p.enabled {
		return http.ProxyFromEnvironment // 使用环境变量中的代理设置
	}

	return func(req *http.Request) (*url.URL, error) {
		return p.proxyURL, nil
	}
}

// IsEnabled 检查是否启用代理
func (p *ProxyHandler) IsEnabled() bool {
	return p.enabled
}

// GetProxyURL 获取代理URL
func (p *ProxyHandler) GetProxyURL() string {
	if p.proxyURL == nil {
		return ""
	}
	return p.proxyURL.String()
}

// Update 更新代理配置
func (p *ProxyHandler) Update(proxyURL string) error {
	if proxyURL == "" {
		p.enabled = false
		p.proxyURL = nil
		return nil
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return err
	}

	p.proxyURL = parsedURL
	p.enabled = true
	return nil
}

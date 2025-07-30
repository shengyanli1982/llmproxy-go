package headers

import (
	"net/http"
	"testing"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOperatorInsert 测试插入操作
func TestOperatorInsert(t *testing.T) {
	operator := NewOperator()
	headers := make(http.Header)

	// 测试插入新头部
	op := config.HeaderOpConfig{
		Op:    "insert",
		Key:   "X-Test-Header",
		Value: "test-value",
	}

	err := operator.ProcessSingle(headers, op)
	require.NoError(t, err)
	assert.Equal(t, "test-value", headers.Get("X-Test-Header"))

	// 测试插入已存在的头部（不应该覆盖）
	op2 := config.HeaderOpConfig{
		Op:    "insert",
		Key:   "X-Test-Header",
		Value: "new-value",
	}

	err = operator.ProcessSingle(headers, op2)
	require.NoError(t, err)
	// 值应该保持原来的
	assert.Equal(t, "test-value", headers.Get("X-Test-Header"))
}

// TestOperatorReplace 测试替换操作
func TestOperatorReplace(t *testing.T) {
	operator := NewOperator()
	headers := make(http.Header)

	// 先设置一个头部
	headers.Set("X-Test-Header", "original-value")

	// 测试替换操作
	op := config.HeaderOpConfig{
		Op:    "replace",
		Key:   "X-Test-Header",
		Value: "new-value",
	}

	err := operator.ProcessSingle(headers, op)
	require.NoError(t, err)
	assert.Equal(t, "new-value", headers.Get("X-Test-Header"))

	// 测试替换不存在的头部（应该创建）
	op2 := config.HeaderOpConfig{
		Op:    "replace",
		Key:   "X-New-Header",
		Value: "created-value",
	}

	err = operator.ProcessSingle(headers, op2)
	require.NoError(t, err)
	assert.Equal(t, "created-value", headers.Get("X-New-Header"))
}

// TestOperatorRemove tests remove operation
func TestOperatorRemove(t *testing.T) {
	operator := NewOperator()
	headers := make(http.Header)

	// Set a header first
	headers.Set("X-Test-Header", "test-value")

	// Test remove operation
	op := config.HeaderOpConfig{
		Op:  "remove",
		Key: "X-Test-Header",
	}

	err := operator.ProcessSingle(headers, op)
	require.NoError(t, err)
	assert.Empty(t, headers.Get("X-Test-Header"))

	// Test removing non-existent header (should not error)
	op2 := config.HeaderOpConfig{
		Op:  "remove",
		Key: "X-Nonexistent-Header",
	}

	err = operator.ProcessSingle(headers, op2)
	require.NoError(t, err)
}

// TestOperatorBatchProcess tests batch processing
func TestOperatorBatchProcess(t *testing.T) {
	operator := NewOperator()
	headers := make(http.Header)

	// Define multiple operations
	ops := []config.HeaderOpConfig{
		{Op: "insert", Key: "X-Header-1", Value: "value-1"},
		{Op: "insert", Key: "X-Header-2", Value: "value-2"},
		{Op: "replace", Key: "X-Header-1", Value: "new-value-1"},
		{Op: "remove", Key: "X-Header-2"},
	}

	err := operator.Process(headers, ops)
	require.NoError(t, err)

	// Verify results
	assert.Equal(t, "new-value-1", headers.Get("X-Header-1"))
	assert.Empty(t, headers.Get("X-Header-2"))
}

// TestOperatorErrorHandling tests error handling
func TestOperatorErrorHandling(t *testing.T) {
	operator := NewOperator()

	// Test nil header
	err := operator.ProcessSingle(nil, config.HeaderOpConfig{Op: "insert", Key: "test", Value: "value"})
	assert.Equal(t, ErrNilHeader, err)

	// Test empty key
	headers := make(http.Header)
	err = operator.ProcessSingle(headers, config.HeaderOpConfig{Op: "insert", Key: "", Value: "value"})
	assert.Equal(t, ErrEmptyHeaderKey, err)

	// Test invalid operation
	err = operator.ProcessSingle(headers, config.HeaderOpConfig{Op: "invalid", Key: "test", Value: "value"})
	assert.Error(t, err)
}

// TestProcessor 测试处理器
func TestProcessor(t *testing.T) {
	processor := NewProcessor()
	req, err := http.NewRequest("GET", "http://example.com", nil)
	require.NoError(t, err)

	// 测试ApplyToRequest
	ops := []config.HeaderOpConfig{
		{Op: "insert", Key: "X-Test-1", Value: "value-1"},
		{Op: "insert", Key: "X-Test-2", Value: "value-2"},
	}

	err = processor.ApplyToRequest(req, ops)
	require.NoError(t, err)
	assert.Equal(t, "value-1", req.Header.Get("X-Test-1"))
	assert.Equal(t, "value-2", req.Header.Get("X-Test-2"))

	// Test individual operation methods
	err = processor.InsertHeader(req, "X-Insert", "insert-value")
	require.NoError(t, err)
	assert.Equal(t, "insert-value", req.Header.Get("X-Insert"))

	err = processor.ReplaceHeader(req, "X-Insert", "replaced-value")
	require.NoError(t, err)
	assert.Equal(t, "replaced-value", req.Header.Get("X-Insert"))

	err = processor.RemoveHeader(req, "X-Insert")
	require.NoError(t, err)
	assert.Empty(t, req.Header.Get("X-Insert"))
}

// TestProcessorFromUpstreamConfig 测试从上游配置应用头部操作
func TestProcessorFromUpstreamConfig(t *testing.T) {
	processor := NewProcessor()
	req, err := http.NewRequest("GET", "http://example.com", nil)
	require.NoError(t, err)

	// 创建上游配置
	upstreamConfig := &config.UpstreamConfig{
		Headers: []config.HeaderOpConfig{
			{Op: "insert", Key: "X-Upstream-Header", Value: "upstream-value"},
			{Op: "replace", Key: "User-Agent", Value: "LLMProxy/1.0"},
		},
	}

	err = processor.ApplyFromUpstreamConfig(req, upstreamConfig)
	require.NoError(t, err)
	assert.Equal(t, "upstream-value", req.Header.Get("X-Upstream-Header"))
	assert.Equal(t, "LLMProxy/1.0", req.Header.Get("User-Agent"))

	// 测试nil配置
	err = processor.ApplyFromUpstreamConfig(req, nil)
	assert.NoError(t, err)

	// 测试没有headers的配置
	emptyConfig := &config.UpstreamConfig{}
	err = processor.ApplyFromUpstreamConfig(req, emptyConfig)
	assert.NoError(t, err)
}

// Additional comprehensive tests

func TestOperatorCaseInsensitiveHeaders(t *testing.T) {
	operator := NewOperator()
	headers := make(http.Header)

	// Set header with different case
	headers.Set("Content-Type", "text/plain")

	// Test replace with different case
	op := config.HeaderOpConfig{
		Op:    "replace",
		Key:   "content-type",
		Value: "application/json",
	}

	err := operator.ProcessSingle(headers, op)
	require.NoError(t, err)
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
	assert.Equal(t, "application/json", headers.Get("content-type"))
}

func TestOperatorMultipleValues(t *testing.T) {
	operator := NewOperator()
	headers := make(http.Header)

	// Add multiple values for same header
	headers.Add("Accept-Encoding", "gzip")
	headers.Add("Accept-Encoding", "deflate")

	// Test that remove removes all values
	op := config.HeaderOpConfig{
		Op:  "remove",
		Key: "Accept-Encoding",
	}

	err := operator.ProcessSingle(headers, op)
	require.NoError(t, err)
	assert.Empty(t, headers.Get("Accept-Encoding"))
	assert.Empty(t, headers.Values("Accept-Encoding"))
}

func TestProcessorNilRequest(t *testing.T) {
	processor := NewProcessor()

	ops := []config.HeaderOpConfig{
		{Op: "insert", Key: "X-Test", Value: "value"},
	}

	err := processor.ApplyToRequest(nil, ops)
	assert.Error(t, err)
}

func TestProcessorEmptyOperations(t *testing.T) {
	processor := NewProcessor()
	req, err := http.NewRequest("GET", "http://example.com", nil)
	require.NoError(t, err)

	// Test with empty operations slice
	err = processor.ApplyToRequest(req, []config.HeaderOpConfig{})
	assert.NoError(t, err)

	// Test with nil operations
	err = processor.ApplyToRequest(req, nil)
	assert.NoError(t, err)
}

func TestProcessorSpecialCharacters(t *testing.T) {
	processor := NewProcessor()
	req, err := http.NewRequest("GET", "http://example.com", nil)
	require.NoError(t, err)

	// Test headers with special characters
	ops := []config.HeaderOpConfig{
		{Op: "insert", Key: "X-Special-Chars", Value: "value with spaces and símböls"},
		{Op: "insert", Key: "X-Unicode", Value: "测试中文"},
		{Op: "insert", Key: "X-Empty-Value", Value: ""},
	}

	err = processor.ApplyToRequest(req, ops)
	require.NoError(t, err)
	assert.Equal(t, "value with spaces and símböls", req.Header.Get("X-Special-Chars"))
	assert.Equal(t, "测试中文", req.Header.Get("X-Unicode"))
	assert.Equal(t, "", req.Header.Get("X-Empty-Value"))
}

func TestOperatorConcurrentAccess(t *testing.T) {
	operator := NewOperator()

	// Test concurrent access to the same operator
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			headers := make(http.Header)
			op := config.HeaderOpConfig{
				Op:    "insert",
				Key:   "X-Goroutine",
				Value: "goroutine-value",
			}

			err := operator.ProcessSingle(headers, op)
			assert.NoError(t, err)
			assert.Equal(t, "goroutine-value", headers.Get("X-Goroutine"))
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Benchmark tests
func BenchmarkOperatorInsert(b *testing.B) {
	operator := NewOperator()
	op := config.HeaderOpConfig{
		Op:    "insert",
		Key:   "X-Benchmark",
		Value: "benchmark-value",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		headers := make(http.Header)
		operator.ProcessSingle(headers, op)
	}
}

func BenchmarkOperatorReplace(b *testing.B) {
	operator := NewOperator()
	op := config.HeaderOpConfig{
		Op:    "replace",
		Key:   "User-Agent",
		Value: "LLMProxy/1.0",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		headers := make(http.Header)
		headers.Set("User-Agent", "curl/7.68.0")
		operator.ProcessSingle(headers, op)
	}
}

func BenchmarkProcessorApplyToRequest(b *testing.B) {
	processor := NewProcessor()
	ops := []config.HeaderOpConfig{
		{Op: "insert", Key: "X-Request-ID", Value: "req-123"},
		{Op: "replace", Key: "User-Agent", Value: "LLMProxy/1.0"},
		{Op: "remove", Key: "X-Debug"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		req.Header.Set("User-Agent", "curl/7.68.0")
		req.Header.Set("X-Debug", "true")
		processor.ApplyToRequest(req, ops)
	}
}

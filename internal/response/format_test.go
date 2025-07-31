package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shengyanli1982/toolkit/pkg/httptool"
	"github.com/stretchr/testify/assert"
)

// TestResponseFormat 测试响应格式是否符合httptool标准
// TestResponseFormat tests if response format conforms to httptool standard
func TestResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success Response Format", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		testData := map[string]interface{}{
			"message": "success",
			"id":      123,
		}

		Success(testData).JSON(c, http.StatusOK)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

		var response httptool.BaseHttpResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, int64(CodeSuccess), response.Code)
		assert.Equal(t, "", response.ErrorMessage)
		assert.Nil(t, response.ErrorDetail)
		assert.NotNil(t, response.Data)
	})

	t.Run("Error Response Format", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		Error(CodeBadRequest, "参数错误").JSON(c, http.StatusBadRequest)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response httptool.BaseHttpResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, int64(CodeBadRequest), response.Code)
		assert.Equal(t, "参数错误", response.ErrorMessage)
		assert.Nil(t, response.ErrorDetail)
		assert.Nil(t, response.Data)
	})

	t.Run("Error Response With Detail", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		detail := map[string]string{"field": "username"}
		Error(CodeBadRequest, "参数错误").WithDetail(detail).JSON(c, http.StatusBadRequest)

		var response httptool.BaseHttpResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, int64(CodeBadRequest), response.Code)
		assert.Equal(t, "参数错误", response.ErrorMessage)
		assert.NotNil(t, response.ErrorDetail)
	})

	t.Run("Paginated Response Format", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		testData := []map[string]interface{}{
			{"id": 1, "name": "test1"},
			{"id": 2, "name": "test2"},
		}

		Paginated(testData, 100, 1, 10, false).JSON(c, http.StatusOK)

		assert.Equal(t, http.StatusOK, w.Code)

		var response httptool.HttpResponsePaginated
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, int64(CodeSuccess), response.BaseHttpResponse.Code)
		assert.Equal(t, int64(100), response.TotalCount)
		assert.Equal(t, int64(1), response.PageIndex)
		assert.Equal(t, int64(10), response.PageSize)
		assert.False(t, response.Desc)
		assert.NotNil(t, response.BaseHttpResponse.Data)
	})
}

// TestObjectPoolCorrectness 测试对象池的正确性
func TestObjectPoolCorrectness(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Object Reset", func(t *testing.T) {
		// 创建一个对象并设置一些数据
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		
		Error(CodeBadRequest, "first error").WithDetail("detail1").JSON(c1, http.StatusBadRequest)
		
		// 创建另一个对象，应该是干净的
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		
		Success("clean data").JSON(c2, http.StatusOK)
		
		var response httptool.BaseHttpResponse
		err := json.Unmarshal(w2.Body.Bytes(), &response)
		assert.NoError(t, err)
		
		// 验证第二个响应是干净的，没有第一个响应的数据残留
		assert.Equal(t, int64(CodeSuccess), response.Code)
		assert.Equal(t, "", response.ErrorMessage)
		assert.Nil(t, response.ErrorDetail)
		assert.Equal(t, "clean data", response.Data)
	})

	t.Run("Concurrent Object Pool Usage", func(t *testing.T) {
		const numGoroutines = 100
		const numIterations = 10
		
		var wg sync.WaitGroup
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < numIterations; j++ {
					w := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(w)
					
					// 交替创建成功和错误响应
					if (id+j)%2 == 0 {
						Success(map[string]int{"id": id, "iteration": j}).JSON(c, http.StatusOK)
					} else {
						Error(CodeBadRequest, "test error").WithDetail(map[string]int{"id": id, "iteration": j}).JSON(c, http.StatusBadRequest)
					}
					
					// 验证响应正确性
					var response httptool.BaseHttpResponse
					err := json.Unmarshal(w.Body.Bytes(), &response)
					assert.NoError(t, err)
					
					if (id+j)%2 == 0 {
						assert.Equal(t, int64(CodeSuccess), response.Code)
						assert.NotNil(t, response.Data)
					} else {
						assert.Equal(t, int64(CodeBadRequest), response.Code)
						assert.Equal(t, "test error", response.ErrorMessage)
						assert.NotNil(t, response.ErrorDetail)
					}
				}
			}(i)
		}
		
		wg.Wait()
	})
}

// BenchmarkResponseBuilder_WithoutPool 基准测试：不使用对象池的响应构建器
func BenchmarkResponseBuilder_WithoutPool(b *testing.B) {
	gin.SetMode(gin.TestMode)
	
	testData := map[string]interface{}{
		"message": "benchmark test",
		"id":      12345,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		
		// 模拟不使用对象池的旧实现
		builder := &ResponseBuilder{
			response: &httptool.BaseHttpResponse{
				Code: CodeSuccess,
				Data: testData,
			},
		}
		c.JSON(http.StatusOK, builder.response)
	}
}

// BenchmarkResponseBuilder_WithPool 基准测试：使用对象池的响应构建器
func BenchmarkResponseBuilder_WithPool(b *testing.B) {
	gin.SetMode(gin.TestMode)
	
	testData := map[string]interface{}{
		"message": "benchmark test",
		"id":      12345,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		
		Success(testData).JSON(c, http.StatusOK)
	}
}

// BenchmarkResponseBuilder_Concurrent 基准测试：并发使用对象池
func BenchmarkResponseBuilder_Concurrent(b *testing.B) {
	gin.SetMode(gin.TestMode)
	
	testData := map[string]interface{}{
		"message": "concurrent benchmark test",
		"id":      12345,
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			
			Success(testData).JSON(c, http.StatusOK)
		}
	})
}

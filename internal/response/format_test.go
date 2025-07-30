package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

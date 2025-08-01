package metrics

import (
	"fmt"
	"strconv"
	"testing"
)

// BenchmarkStatusCodeFormatting 比较不同状态码格式化方法的性能
func BenchmarkStatusCodeFormatting(b *testing.B) {
	statusCodes := []int{200, 404, 500, 299} // 包含常见和不常见的状态码

	b.Run("fmt.Sprintf", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, code := range statusCodes {
				_ = fmt.Sprintf("%d", code)
			}
		}
	})

	b.Run("strconv.Itoa", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, code := range statusCodes {
				_ = strconv.Itoa(code)
			}
		}
	})

	b.Run("formatStatusCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, code := range statusCodes {
				_ = formatStatusCode(code)
			}
		}
	})
}

// BenchmarkCommonStatusCodes 测试常见状态码的性能
func BenchmarkCommonStatusCodes(b *testing.B) {
	commonCodes := []int{200, 404, 500, 401, 403, 502, 503}

	b.Run("fmt.Sprintf_common", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, code := range commonCodes {
				_ = fmt.Sprintf("%d", code)
			}
		}
	})

	b.Run("formatStatusCode_common", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, code := range commonCodes {
				_ = formatStatusCode(code)
			}
		}
	})
}

// BenchmarkUncommonStatusCodes 测试不常见状态码的性能
func BenchmarkUncommonStatusCodes(b *testing.B) {
	uncommonCodes := []int{418, 451, 299, 226}

	b.Run("fmt.Sprintf_uncommon", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, code := range uncommonCodes {
				_ = fmt.Sprintf("%d", code)
			}
		}
	})

	b.Run("formatStatusCode_uncommon", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, code := range uncommonCodes {
				_ = formatStatusCode(code)
			}
		}
	})
}

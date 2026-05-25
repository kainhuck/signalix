package logger

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

// HTTPMiddleware 标准 net/http 日志中间件
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 获取或生成 Request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// 注入日志字段到 context
		ctx := WithContextFields(r.Context(),
			String("request_id", requestID),
			String("method", r.Method),
			String("path", r.URL.Path),
			String("remote_addr", r.RemoteAddr),
		)

		// 设置响应头
		w.Header().Set("X-Request-ID", requestID)

		L(ctx).Info("request started")

		// 包装 ResponseWriter
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// 处理请求
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		// 记录请求完成
		L(ctx).Info("request completed",
			Int("status", wrapped.statusCode),
			Duration("latency", time.Since(start)),
			Int("bytes", wrapped.bytesWritten),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

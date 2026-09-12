package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestReadyzReportsUninitializedDependencies 验证 readyz 的失败聚合：
// 资源未初始化（helpers.MysqlClientLLM / RedisClient 为 nil）时必须返回 503 并指明失败项。
func TestReadyzReportsUninitializedDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/readyz", nil)

	readyzHandler(ctx)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz code = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"status":"unavailable"`) {
		t.Fatalf("readyz body missing unavailable status: %s", body)
	}
	if !strings.Contains(body, "not initialized") {
		t.Fatalf("readyz body should point out uninitialized dependencies: %s", body)
	}
}

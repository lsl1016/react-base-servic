package workspace

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"react-base-service/conf"
	"react-base-service/golib/env"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
)

// TestDiagLoadManual 诊断用：手动走一遍 Load→Release（不依赖模型）。
func TestDiagLoadManual(t *testing.T) {
	if os.Getenv("WS_DIAG") == "" {
		t.Skip("set WS_DIAG=1")
	}
	env.SetRootPath("../..")
	conf.InitConf()
	helpers.InitMysql()
	// repo-mcp stdio 适配器按相对路径 bin/repo-mcp.exe 拉起：诊断进程切到仓库根。
	if err := os.Chdir("../.."); err != nil {
		t.Fatal(err)
	}
	if wd, err := os.Getwd(); err == nil {
		t.Logf("CWD=%s", wd)
	}
	gin.SetMode(gin.TestMode)
	ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/diag", nil)
	helpers.SetUserName(ginCtx, "diag")

	m := Default()
	alloc, err := m.Load(ginCtx, "run_diag", "demo-app", "react-base-service", "")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	t.Logf("alloc: service=%s commit=%s path=%s server=%s", alloc.Service, alloc.Commit, alloc.Path, alloc.MCPServerName)
	if data, err := os.ReadFile(alloc.Path + "/README.md"); err == nil {
		t.Logf("README head: %q", string(data[:40]))
	}
	m.ReleaseRun(ginCtx, "run_diag")
}

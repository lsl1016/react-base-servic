package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestToolUserPolicyRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	InitLLMRouter(engine.Group("/react-base-service"))

	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	prefix := "/react-base-service/tool/whitelist"
	want := []string{
		http.MethodPost + " " + prefix + "/create",
		http.MethodPost + " " + prefix + "/update",
		http.MethodPost + " " + prefix + "/delete",
		http.MethodPost + " " + prefix + "/detail",
		http.MethodPost + " " + prefix + "/list",
	}
	for _, route := range want {
		if _, ok := routes[route]; !ok {
			t.Errorf("tool user policy route not registered: %s", route)
		}
	}
}

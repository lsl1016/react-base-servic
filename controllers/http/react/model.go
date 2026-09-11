package react

import (
	"react-base-service/components"
	"react-base-service/components/params"
	"react-base-service/conf"

	"github.com/gin-gonic/gin"
)

// GetModels 返回 ReAct 前端可选择的模型列表和默认模型。
func GetModels(ctx *gin.Context) {
	cfg := conf.GetReactRuntimeConfig().Models
	models := make([]params.ReactModelInfo, 0, len(cfg.Available))
	for _, item := range cfg.Available {
		models = append(models, params.ReactModelInfo{
			ModelKey:     item.ModelKey,
			ModelVersion: item.ModelVersion,
			DisplayName:  item.DisplayName,
		})
	}
	components.RenderJsonSucc(ctx, params.ReactModelsResp{
		Models: models,
		DefaultModel: params.ReactModelInfo{
			ModelKey:     cfg.Default.ModelKey,
			ModelVersion: cfg.Default.ModelVersion,
			DisplayName:  cfg.Default.DisplayName,
		},
	})
}

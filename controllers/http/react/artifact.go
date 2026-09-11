package react

import (
	"mime"
	"net/http"
	"strings"

	reactService "react-base-service/service/react"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// GetArtifact 下载 python_exec 生成的产物文件。文件存于 COS 私有桶，经本接口按 artifactId 读取回吐，
// 桶不直接对外暴露，资源生命周期由后端自管。
// @Summary ReAct Python 产物下载
// @Tags React
// @Produce application/octet-stream
// @Param artifactId path string true "产物ID"
// @Router /react/artifact/{artifactId} [get]
func GetArtifact(ctx *gin.Context) {
	artifactID := strings.TrimSpace(ctx.Param("artifactId"))
	if artifactID == "" {
		ctx.String(http.StatusNotFound, "artifact not found")
		return
	}

	loaded, err := reactService.LoadArtifactForDownload(ctx, artifactID)
	if err != nil {
		zlog.Errorf(ctx, "[React.GetArtifact] 读取产物失败: artifactId=%s err=%v", artifactID, err)
		ctx.String(http.StatusInternalServerError, "failed to load artifact")
		return
	}
	if loaded == nil {
		ctx.String(http.StatusNotFound, "artifact not found")
		return
	}

	fileName := strings.TrimSpace(loaded.FileName)
	if fileName == "" {
		fileName = artifactID
	}
	if disposition := mime.FormatMediaType("inline", map[string]string{"filename": fileName}); disposition != "" {
		ctx.Header("Content-Disposition", disposition)
	}
	ctx.Data(http.StatusOK, loaded.MimeType, loaded.Data)
}

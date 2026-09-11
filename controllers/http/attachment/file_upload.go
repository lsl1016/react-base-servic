package attachment

import (
	"react-base-service/components"
	"react-base-service/components/params"
	"react-base-service/helpers"
	skillchatfile "react-base-service/service/skillchatfile"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// UploadChatFile 上传 ReAct 对话附件（仅 csv/md/txt，单文件最大 50MB）
// @Summary      上传 ReAct 附件
// @Description  仅支持 csv、md、txt，单文件最大 50MB，支持 UTF-8、带 BOM 的 UTF-16 和 GB18030 编码。上传成功后返回 fileId，供后续 ReAct run 的 attachments 引用。
// @Tags         attachment
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "文本文件（.csv、.md 或 .txt）"
// @Success      200  {object} components.DefaultRenderWithTrace{data=params.ChatFileUploadResp}
// @Failure      400  {object} components.DefaultRenderWithTrace
// @Router       /api/chat/files/upload [post]
func UploadChatFile(ctx *gin.Context) {
	userName := helpers.GetUserName(ctx)
	if userName == "" || userName == "unknown" || userName == "system" {
		components.RenderJsonFail(ctx, components.ParamInvalidf("未获取到操作人用户名"))
		return
	}

	file, err := ctx.FormFile("file")
	if err != nil {
		components.RenderJsonFail(ctx, components.ErrorParamInvalid.Sprintf("file不能为空"))
		return
	}

	meta, err := skillchatfile.SaveChatUploadFile(ctx, userName, file)
	if err != nil {
		zlog.Errorf(ctx, "[Attachment.UploadChatFile] upload failed: %v", err)
		components.RenderJsonFail(ctx, err)
		return
	}

	components.RenderJsonSucc(ctx, params.ChatFileUploadResp{
		FileID:     meta.FileID,
		FileName:   meta.FileName,
		Ext:        meta.Ext,
		Size:       meta.Size,
		UploadedAt: meta.UploadedAt,
	})
}

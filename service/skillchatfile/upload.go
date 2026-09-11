package skillchatfile

import (
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"react-base-service/components"
	"react-base-service/conf"
	"react-base-service/helpers"
	model "react-base-service/models/llm"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// SaveChatUploadFile 校验 csv/md/txt、≤50MB，检测字符集（UTF-8/UTF-16/GB18030）后统一转为 UTF-8 上传 COS 并写入元数据
func SaveChatUploadFile(ctx *gin.Context, owner string, file *multipart.FileHeader) (*ChatFileMeta, error) {
	if file == nil {
		return nil, components.ErrorParamInvalid.Sprintf("file不能为空")
	}
	if owner == "" {
		return nil, components.ErrorParamInvalid.Sprintf("owner不能为空")
	}

	filename := filepath.Base(file.Filename)
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if !IsSupportedChatFileExtension(ext) {
		return nil, components.ErrorParamInvalid.Sprintf("仅支持 csv / md / txt 文件")
	}

	if file.Size > ChatFileUploadMaxBytes {
		return nil, components.ParamInvalidf("文件大小不能超过 50MB")
	}

	src, err := file.Open()
	if err != nil {
		return nil, components.ErrorParamInvalid.Sprintf("读取上传文件失败: %v", err)
	}
	defer func() { _ = src.Close() }()

	lr := io.LimitReader(src, ChatFileUploadMaxBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, components.ErrorParamInvalid.Sprintf("读取上传文件失败: %v", err)
	}
	if len(data) > ChatFileUploadMaxBytes {
		return nil, components.ParamInvalidf("文件大小不能超过 50MB")
	}
	originalSize := int64(len(data))
	charset, err := DetectChatFileCharset(data)
	if err != nil {
		return nil, err
	}
	zlog.Infof(ctx, "[skillchatfile] 上传文件检测编码: owner=%s, fileName=%s, ext=%s, originalCharset=%s, sizeBytes=%d", owner, filename, ext, charset, len(data))
	// 统一转成 UTF-8 再存 COS：让 COS 上的文本附件永远是 UTF-8（并剥掉可能的 BOM），
	// 下游（read_attachment、python 沙箱 cos_ref 等）无需再处理编码，避免非 UTF-8 附件解码失败。
	utf8Text, err := DecodeChatFileToUTF8(data, charset)
	if err != nil {
		return nil, err
	}
	data = []byte(utf8Text)
	charset = CharsetUTF8

	if err := helpers.EnsureCos(); err != nil {
		return nil, components.ErrorSystemError
	}
	bucket := strings.TrimSpace(conf.RConf.Cos.Bucket)
	if bucket == "" {
		zlog.Errorf(ctx, "[skillchatfile] cos bucket 未配置")
		return nil, components.ErrorSystemError
	}

	fileID := newFileID()
	cleanName := sanitizeFileName(filename)
	dateStr := time.Now().Format("2006-01-02")
	cosPrefix := buildChatFileCOSPrefix(conf.RConf.Cos.PathPrefix)
	cosKey := fmt.Sprintf("%s/%s/%s/%s", cosPrefix, dateStr, fileID, cleanName)

	mediaType := "text/plain"
	switch ext {
	case extMd:
		mediaType = "text/markdown"
	case extCsv:
		mediaType = "text/csv"
	}
	contentType := fmt.Sprintf("%s; charset=%s", mediaType, charset)

	if err := helpers.CosClient.UploadData(ctx.Request.Context(), data, cosKey, contentType); err != nil {
		zlog.Errorf(ctx, "[skillchatfile] 上传 COS 失败: cosKey=%s err=%v", cosKey, err)
		return nil, components.ErrorSystemError
	}

	cosURI := fmt.Sprintf("cos://%s/%s", bucket, cosKey)
	uploadedAt := time.Now().Format("2006-01-02 15:04:05")

	meta := &ChatFileMeta{
		FileID:     fileID,
		Owner:      owner,
		FileName:   filename,
		Ext:        ext,
		MimeType:   contentType,
		Charset:    charset,
		Size:       originalSize,
		CosKey:     cosKey,
		CosURI:     cosURI,
		Status:     "parsed",
		UploadedAt: uploadedAt,
	}
	if err := SaveMeta(ctx, meta); err != nil {
		return nil, err
	}
	if err := model.CreateChatFileRecord(ctx, &model.ChatFileRecord{
		FileID:   meta.FileID,
		Owner:    meta.Owner,
		FileName: meta.FileName,
		Ext:      meta.Ext,
		MimeType: meta.MimeType,
		Charset:  meta.Charset,
		Size:     meta.Size,
		CosKey:   meta.CosKey,
		CosURI:   meta.CosURI,
		Status:   meta.Status,
	}); err != nil {
		return nil, err
	}
	return meta, nil
}

func sanitizeFileName(name string) string {
	n := filepath.Base(strings.TrimSpace(name))
	if n == "" {
		return "unnamed.txt"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_")
	return replacer.Replace(n)
}

package skillchatfile

import (
	"strings"

	"github.com/google/uuid"
)

const (
	// ChatFileMaxBytes 可直接注入模型上下文的单/总文本上限（200KB）。
	// 仅约束"把内容读进上下文"的路径（skill 老管线、ReAct read_attachment），不是上传上限。
	ChatFileMaxBytes = 200 * 1024
	// ChatFileUploadMaxBytes 单文件上传上限（50MB）。大文件不进上下文，只按 fileId 引用交给 python 分析。
	ChatFileUploadMaxBytes = 50 * 1024 * 1024
	// MetaTTLSeconds 文件元数据在 Redis 中的保留时间（24 小时）
	MetaTTLSeconds = 24 * 60 * 60
	redisKeyPrefix = "llm:chat:file:meta:"
)

// defaultChatFileCOSRoot pathPrefix 未配置时的兜底，与 resource.yaml 默认 react-base/llm 一致
const defaultChatFileCOSRoot = "react-base/llm"

// buildChatFileCOSPrefix 基于 conf 中 cos.pathPrefix（如 react-base/llm），拼接 chat-files 子目录，统一复用顶层前缀、避免硬编码重复。
func buildChatFileCOSPrefix(raw string) string {
	prefix := strings.Trim(strings.TrimSpace(raw), "/")
	if prefix == "" {
		prefix = defaultChatFileCOSRoot
	}
	if strings.HasSuffix(strings.ToLower(prefix), "/chat-files") {
		return prefix
	}
	return prefix + "/chat-files"
}

const (
	extTxt = "txt"
	extMd  = "md"
	extCsv = "csv"
)

// IsSupportedChatFileExtension 判断文件是否可作为 Chat/ReAct 文本附件。
func IsSupportedChatFileExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case extTxt, extMd, extCsv:
		return true
	default:
		return false
	}
}

// ChatFileMeta 文件元数据（存 Redis，供后续 chat 引用）
type ChatFileMeta struct {
	FileID     string `json:"fileId"`
	Owner      string `json:"owner"`
	FileName   string `json:"fileName"`
	Ext        string `json:"ext"`
	MimeType   string `json:"mimeType"`
	Charset    string `json:"charset,omitempty"`
	Size       int64  `json:"size"`
	CosKey     string `json:"cosKey"`
	CosURI     string `json:"cosUri"`
	Status     string `json:"status"`
	UploadedAt string `json:"uploadedAt"`
}

func metaRedisKey(fileID string) string {
	return redisKeyPrefix + fileID
}

func newFileID() string {
	return "file_" + strings.ReplaceAll(uuid.New().String(), "-", "")
}

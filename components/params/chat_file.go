package params

// ChatFileUploadResp Chat 文件上传响应（不返回 COS 路径，前端仅用 fileId 参与后续 chat）
type ChatFileUploadResp struct {
	FileID     string `json:"fileId"`
	FileName   string `json:"fileName"`
	Ext        string `json:"ext"`
	Size       int64  `json:"size"`
	UploadedAt string `json:"uploadedAt"`
}

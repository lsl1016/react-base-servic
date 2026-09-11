package react

import (
	"encoding/json"
	"fmt"
	"strings"

	llm "react-base-service/api/llm"
)

const (
	// inspectAttachmentHeadLines 头部预览返回的最大行数
	inspectAttachmentHeadLines = 30
	// inspectAttachmentHeadMaxRunes 头部预览的字符预算上限（32KB，约为 inline 预览的 2 倍，可容纳上千列的宽表头）。
	// inspect 只是"廉价瞄一眼、够写第一版代码"，不求全：截断了也没关系，权威 schema 由 python 里的 pandas
	// （df.columns / df.dtypes / df.shape）拿。这里只兜住病态超长单行/超宽文件的内存。
	inspectAttachmentHeadMaxRunes = 32 * 1024
)

func readAttachmentToolDefinition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        metaToolReadAttachment,
		Description: "读取一个上传附件的文本内容到上下文，用于需要理解、总结、改写、翻译或逐行判断文件内容的任务。fileId 来自 attachments 清单。内容较大时自动返回预览并生成 resultRef，可用 read_tool_result 翻页。若任务是统计/聚合/计算等数据处理，改用 python_exec 按 fileId 引用分析，不要用本工具把大文件读进上下文。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description": stringSchema("本次工具调用的简短描述，用于向用户说明为什么调用该内部工具或正在做什么。"),
				"fileId":      stringSchema("附件 fileId，来自 attachments 清单。"),
			},
			"required":             []string{"description", "fileId"},
			"additionalProperties": false,
		},
	}
}

func inspectAttachmentToolDefinition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        metaToolInspectAttachment,
		Description: "预览一个上传附件的头部内容，用于写 python 分析代码前了解文件长什么样。返回文件头部的前若干行原始文本、总行数和文件大小，不做任何解析——是否有表头、用什么分隔符、每列是什么类型，都由你看样例自行判断（或在 python 里用 pandas 探查）。适用于 csv/txt/md。fileId 来自 attachments 清单。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description": stringSchema("本次工具调用的简短描述，用于向用户说明为什么调用该内部工具或正在做什么。"),
				"fileId":      stringSchema("附件 fileId，来自 attachments 清单。"),
			},
			"required":             []string{"description", "fileId"},
			"additionalProperties": false,
		},
	}
}

// readAttachment 下载并解码附件文本内容，结果交由 executeInternalTool 走标准截断管线（大结果自动 preview+resultRef）。
func (s *reactEngineState) readAttachment(input json.RawMessage) (string, bool, error) {
	var req struct {
		FileID string `json:"fileId"`
	}
	_ = json.Unmarshal(input, &req)
	fileID := strings.TrimSpace(req.FileID)
	if fileID == "" {
		return "", true, fmt.Errorf("read_attachment fileId is required")
	}
	record, err := resolveAttachmentRecord(s.ctx.Request.Context(), s.req.userName, fileID)
	if err != nil {
		return "", true, err
	}
	content, err := downloadDecodeAttachment(s.ctx.Request.Context(), record)
	if err != nil {
		return "", true, err
	}
	payload := map[string]interface{}{
		"fileId":   record.FileID,
		"fileName": record.FileName,
		"ext":      record.Ext,
		"content":  content,
	}
	data, _ := json.Marshal(payload)
	return string(data), false, nil
}

// inspectAttachment 返回附件头部原始预览：前若干行 + 总行数 + 大小，不解析、不推类型、不假设表头。
func (s *reactEngineState) inspectAttachment(input json.RawMessage) (string, bool, error) {
	var req struct {
		FileID string `json:"fileId"`
	}
	_ = json.Unmarshal(input, &req)
	fileID := strings.TrimSpace(req.FileID)
	if fileID == "" {
		return "", true, fmt.Errorf("inspect_attachment fileId is required")
	}
	record, err := resolveAttachmentRecord(s.ctx.Request.Context(), s.req.userName, fileID)
	if err != nil {
		return "", true, err
	}
	content, err := downloadDecodeAttachment(s.ctx.Request.Context(), record)
	if err != nil {
		return "", true, err
	}
	headLines, totalLines, truncated := attachmentHeadPreview(content, inspectAttachmentHeadLines, inspectAttachmentHeadMaxRunes)
	payload := map[string]interface{}{
		"fileId":     record.FileID,
		"fileName":   record.FileName,
		"ext":        record.Ext,
		"sizeBytes":  record.Size,
		"totalLines": totalLines,
		"headLines":  headLines,
		"note":       "以下 headLines 为文件头部原始行，未做任何解析。请据此自行判断是否有表头、分隔符和每列含义/类型；需要权威的列类型时在 python 中用 pandas 读取后查看。",
	}
	if truncated {
		payload["headTruncated"] = true
	}
	data, _ := json.Marshal(payload)
	return string(data), false, nil
}

// attachmentHeadPreview 取文件头部的前 maxLines 行（并受 maxRunes 字符预算约束，含行内截断），
// 返回头部行、总行数和是否被截断。
func attachmentHeadPreview(content string, maxLines, maxRunes int) ([]string, int, bool) {
	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return []string{}, 0, false
	}
	lines := strings.Split(trimmed, "\n")
	totalLines := len(lines)

	head := make([]string, 0, maxLines)
	budget := maxRunes
	truncated := false
	for i, line := range lines {
		if i >= maxLines || budget <= 0 {
			truncated = true
			break
		}
		runes := []rune(line)
		if len(runes) > budget {
			head = append(head, string(runes[:budget]))
			budget = 0
			truncated = true
			break
		}
		head = append(head, line)
		budget -= len(runes)
	}
	if len(head) < totalLines {
		truncated = true
	}
	return head, totalLines, truncated
}

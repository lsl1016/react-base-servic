// Package mcpserver 提供零依赖的 MCP stdio 服务端框架（JSON-RPC over stdin/stdout）。
//
// 实现 initialize / ping / tools/list / tools/call 四个方法，足够作为被
// react-base-service MCP 客户端（service/mcpclient）拉起的子进程服务器使用。
// 传输无鉴权：仅适用于本机受信环境，由宿主进程决定拉起方式。
package mcpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// Tool 声明一个 MCP 工具；Handler 在 tools/call 时执行。
type Tool struct {
	Name        string                            `json:"name"`
	Description string                            `json:"description"`
	InputSchema map[string]any                    `json:"inputSchema"`
	Handler     func(map[string]any) (any, error) `json:"-"`
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

// Server 是一个 stdio MCP 服务器。
type Server struct {
	name  string
	tools map[string]Tool
}

// New 用工具集合构造服务器。
func New(name string, tools []Tool) *Server {
	m := make(map[string]Tool, len(tools))
	for _, t := range tools {
		m[t.Name] = t
	}
	return &Server{name: name, tools: m}
}

// Serve 阻塞地从 stdin 读取 JSON-RPC 请求并写响应到 stdout，直到 stdin 关闭。
func (s *Server) Serve() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 2<<20)
	enc := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		// JSON-RPC notification 不需要响应。
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue
		}
		id := decodeID(req.ID)
		res := response{JSONRPC: "2.0", ID: id}
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			_ = json.Unmarshal(req.Params, &p)
			if p.ProtocolVersion == "" {
				p.ProtocolVersion = "2025-06-18"
			}
			res.Result = map[string]any{
				"protocolVersion": p.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": s.name, "version": "0.1.0"},
			}
		case "ping":
			res.Result = map[string]any{}
		case "tools/list":
			list := make([]Tool, 0, len(s.tools))
			for _, tool := range s.tools {
				list = append(list, tool)
			}
			res.Result = map[string]any{"tools": list}
		case "tools/call":
			res.Result = s.call(req.Params)
		default:
			res.Error = map[string]any{"code": -32601, "message": "method not found"}
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *Server) call(raw json.RawMessage) any {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return toolResult(nil, fmt.Errorf("invalid tool params:%w", err))
	}
	tool, ok := s.tools[p.Name]
	if !ok {
		return toolResult(nil, fmt.Errorf("unknown tool:%s", p.Name))
	}
	result, err := tool.Handler(p.Arguments)
	return toolResult(result, err)
}

func toolResult(v any, err error) map[string]any {
	if err != nil {
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		}
	}
	data, _ := json.Marshal(v)
	return map[string]any{
		"content":           []map[string]any{{"type": "text", "text": string(data)}},
		"structuredContent": v,
		"isError":           false,
	}
}

func decodeID(raw json.RawMessage) any {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

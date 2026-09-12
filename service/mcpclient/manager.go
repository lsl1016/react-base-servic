package mcpclient

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"react-base-service/conf"
	"react-base-service/golib/zlog"
	model "react-base-service/models/llm"

	"github.com/gin-gonic/gin"
)

// stderrWriter 把子进程 stderr 透传到服务日志。
type stderrWriter struct{ server string }

func (w stderrWriter) Write(p []byte) (int, error) {
	text := strings.TrimRight(string(p), "\r\n")
	if text != "" {
		zlog.Infof(nil, "[MCP.%s] %s", w.server, text)
	}
	return len(p), nil
}

// RegistryTool 是 tools/list 拿到的一个 MCP 工具定义。
type RegistryTool struct {
	Server      string
	Tool        string
	Description string
	InputSchema map[string]any
}

// ToolConfigJSON 生成写入 tblLlmTool.config 的 JSON（tool_type=mcp）。
// inputSchema 供 get_tool 校验与参数透出；mcpServer/mcpTool 供执行分发。
func ToolConfigJSON(server, tool string, inputSchema map[string]any) (string, error) {
	data, err := json.Marshal(map[string]any{
		"mcpServer":  server,
		"mcpTool":    tool,
		"inputSchema": orEmptyObject(inputSchema),
	})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func orEmptyObject(schema map[string]any) map[string]any {
	if schema == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return schema
}

// Server 是一个 MCP 服务器的统一抽象：stdio（kind=repo）与 HTTP（kind=http）实现同一接口，
// 注册表同步与 execute_tool 分发对传输形式无感知。
type Server interface {
	Name() string
	Start() error
	Stop() error
	ListTools() ([]RegistryTool, error)
	CallTool(name string, arguments json.RawMessage, timeout time.Duration) (string, error)
}

// Manager 管理全部配置声明的 MCP 服务器客户端。
type Manager struct {
	mu      sync.RWMutex
	clients map[string]Server
}

var defaultManager = &Manager{clients: map[string]Server{}}

// Default 返回全局 Manager。
func Default() *Manager { return defaultManager }

// Bootstrap 按配置拉起全部 MCP 服务器并同步工具注册表。
// 任一服务器失败只记录日志，不影响服务启动与其余服务器。
func Bootstrap(engine *gin.Engine) {
	mcpConf := conf.CustomConf.MCP
	if strings.TrimSpace(mcpConf.CallerKey) == "" || len(mcpConf.Servers) == 0 {
		zlog.Infof(nil, "[MCP] 未配置 mcp.servers，跳过 MCP 客户端启动")
		return
	}

	for _, serverCfg := range mcpConf.Servers {
		client, err := newServer(ServerConfig{
			Name:      serverCfg.Name,
			Kind:      serverCfg.Kind,
			Env:       serverCfg.Env,
			Endpoint:  serverCfg.Endpoint,
			TimeoutMs: serverCfg.TimeoutMs,
		})
		if err != nil {
			zlog.Errorf(nil, "[MCP] 服务器配置无效: %v", err)
			continue
		}
		if err := client.Start(); err != nil {
			zlog.Errorf(nil, "[MCP] 启动 %s 失败: %v", serverCfg.Name, err)
			continue
		}
		defaultManager.mu.Lock()
		defaultManager.clients[client.Name()] = client
		defaultManager.mu.Unlock()
		zlog.Infof(nil, "[MCP] 服务器 %s（kind=%s）已启动", serverCfg.Name, serverCfg.Kind)
	}

	if err := SyncRegistry(mcpConf.CallerKey); err != nil {
		zlog.Errorf(nil, "[MCP] 工具注册表同步失败: %v", err)
	}
}

// newServer 按 kind 构造对应传输的客户端：
// http=手写 Streamable HTTP；http_sdk=官方 MCP Go SDK 版；其余走 stdio 适配器白名单。
func newServer(cfg ServerConfig) (Server, error) {
	kind := strings.ToLower(strings.TrimSpace(cfg.Kind))
	switch kind {
	case "http":
		return NewHTTPClient(strings.TrimSpace(cfg.Name), cfg.Endpoint, cfg.TimeoutMs)
	case "http_sdk", "sdk":
		return NewSDKClient(strings.TrimSpace(cfg.Name), cfg.Endpoint, cfg.TimeoutMs)
	default:
		return NewClient(cfg)
	}
}

// Shutdown 停止全部服务器（stdio 子进程 / HTTP 客户端）。
func Shutdown() {
	defaultManager.mu.Lock()
	defer defaultManager.mu.Unlock()
	for name, client := range defaultManager.clients {
		if err := client.Stop(); err != nil {
			zlog.Errorf(nil, "[MCP] 停止 %s 失败: %v", name, err)
		}
	}
	defaultManager.clients = map[string]Server{}
}

// Call 经全局 Manager 调用某服务器上的工具。
func Call(server, tool string, arguments json.RawMessage, timeout time.Duration) (string, error) {
	defaultManager.mu.RLock()
	client := defaultManager.clients[server]
	defaultManager.mu.RUnlock()
	if client == nil {
		return "", fmt.Errorf("mcp server %q is not configured/running", server)
	}
	return client.CallTool(tool, arguments, timeout)
}

// SyncRegistry 把全部 MCP 服务器的工具清单 upsert 进 tblLlmTool。
// toolId 固定为 mcp_<server>_<tool>，按 toolId 增量更新；routeValues 为 "[]"（空路由通用）。
func SyncRegistry(callerKey string) error {
	defaultManager.mu.RLock()
	clients := make([]Server, 0, len(defaultManager.clients))
	for _, client := range defaultManager.clients {
		clients = append(clients, client)
	}
	defaultManager.mu.RUnlock()

	ctx := &gin.Context{}
	synced := 0
	for _, client := range clients {
		tools, err := client.ListTools()
		if err != nil {
			zlog.Errorf(nil, "[MCP] %s tools/list 失败: %v", client.Name(), err)
			continue
		}
		for _, tool := range tools {
			if err := upsertRegistryTool(ctx, callerKey, client.Name(), tool); err != nil {
				zlog.Errorf(nil, "[MCP] 注册工具 %s_%s 失败: %v", client.Name(), tool.Tool, err)
				continue
			}
			synced++
		}
	}
	zlog.Infof(nil, "[MCP] 注册表同步完成: callerKey=%s, 共 %d 个 MCP 工具", callerKey, synced)
	return nil
}

func upsertRegistryTool(ctx *gin.Context, callerKey, server string, tool RegistryTool) error {
	toolID := fmt.Sprintf("mcp_%s_%s", server, tool.Tool)
	name := fmt.Sprintf("%s_%s", server, tool.Tool)
	description := strings.TrimSpace(tool.Description)
	if description == "" {
		description = fmt.Sprintf("MCP tool %s/%s", server, tool.Tool)
	}
	configJSON, err := ToolConfigJSON(server, tool.Tool, tool.InputSchema)
	if err != nil {
		return err
	}

	existing, err := model.GetToolByToolID(ctx, toolID)
	if err != nil {
		return err
	}
	if existing != nil {
		updates := map[string]interface{}{
			"name":        name,
			"description": description,
			"config":      configJSON,
			"status":      1,
			"updated_by":  "mcp-sync",
		}
		return model.UpdateToolByToolID(ctx, toolID, updates)
	}
	return model.CreateTool(ctx, &model.Tool{
		ToolID:      toolID,
		Name:        name,
		Description: description,
		ToolType:    "mcp",
		CallerKey:   callerKey,
		RouteValues: "[]",
		Config:      configJSON,
		Status:      1,
		CreatedBy:   "mcp-sync",
		UpdatedBy:   "mcp-sync",
	})
}

# MCP 客户端与仓库检索：能力边界

本文说明 react-base-service 内置 MCP 客户端的实现边界、安全模型，以及 `repo` 适配器（代码仓库检索）的能力边界。未列出的能力均视为**未实现**。

## 1. MCP 实现边界

### 1.1 传输：stdio + Streamable HTTP（手写 / 官方 SDK 双实现）

| 项 | 现状 |
|---|---|
| stdio（子进程 + stdin/stdout JSON-RPC，`kind: repo`） | ✅ 已实现 |
| Streamable HTTP 手写版（`kind: http`） | ✅ 已实现 |
| Streamable HTTP 官方 SDK 版（`kind: http_sdk`，[modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) v1.7.0） | ✅ 已实现 |
| 旧版 HTTP+SSE 双端点 | ❌ 未实现 |

- 三种实现共用同一 `Server` 接口，注册表同步与 `execute_tool` 分发对传输与实现形式无感知；
- `http` 与 `http_sdk` 的取舍：手写版零依赖、代码量小、行为完全可控；SDK 版协议兼容性由官方保证（版本协商、SSE 解析、会话管理）、支持全部内容类型（图片/音频/资源的占位降级）。二者均无鉴权、均按次会话、均走同一 SSRF 端点校验，配置仅 `kind` 不同，可按远端服务器的协议严格程度选择；
- 引入 SDK 使项目 Go 版本要求从 1.23 升至 **1.25**（SDK v1.7.0 的最低要求）；
- stdio 单服务器内请求串行；HTTP **按次会话**（每次操作独立完成 initialize → 操作），无连接池、无状态，代价是每次操作多一次握手往返。

### 1.2 协议方法覆盖

仅实现工具相关方法：`initialize`、`notifications/initialized`、`ping`、`tools/list`、`tools/call`。

**不支持**：`resources/*`、`prompts/*`、`sampling/*`、`roots`、`elicitation` 等 MCP 其余能力域。

### 1.3 安全模型（本机受信环境，无鉴权）

- **stdio（kind=repo）**：可执行文件路径是代码内字面量（按 GOOS 固定于 `bin/repo-mcp[.exe]`），配置只能选择 `kind` 与注入环境变量，**不接受任意 command/args**；
- **HTTP（kind=http）**：端点经 **SSRF 校验**（构造时与每次建会话前各校验一次）——仅允许 `http/https`，主机为 IP 字面量时直接判定，为域名时解析后逐一判定；**拒绝环回、私网（RFC1918/ULA）、链路本地、组播、未指定、CGNAT、TEST-NET 及其他保留网段**，域名解析出任一非公网地址即拒绝（缓解 DNS 重绑定）；
- 两种传输均**无任何身份认证**：stdio 是本机管道；HTTP 不发送凭证头，**仅应指向受信的公网端点**，不得将内网 MCP 服务配置进来；
- 单次 `tools/call` 超时：stdio 默认 60s（工具 config `timeout_ms` 可覆盖）；HTTP 由 server 配置 `timeout_ms`（默认 30s）控制；超时按错误返回；
- stdio 子进程 stderr 透传到服务日志；stdio 生命周期随服务启停（`router.Tasks` / `StopTasks`），HTTP 无常驻资源。

### 1.4 工具注册与执行链路

```
服务启动 → 拉起子进程 + initialize → tools/list
        → upsert tblLlmTool（toolId=mcp_<server>_<tool>，toolType=mcp，
          config={mcpServer,mcpTool,inputSchema}，挂 caller_key，route_values="[]"）
模型侧 → 工具索引摘要（system 前缀）→ get_tool 加载参数 → execute_tool
        → 路由到 tools/call → 文本内容回填（大结果自动走 resultRef 分片）
```

边界说明：

- **同步时机是启动时一次性**：运行中服务器新增/变更工具不会自动重同步（重启服务触发；也可手动删改注册表行）；
- 工具在注册表中就是普通 Business Tool 行，**可被工具管理面板启停/修改/删除**，也受用户白名单（ToolUserPolicy）约束；
- 同步失败只记日志，不阻断服务启动。

### 1.5 配置（custom.yaml）

```yaml
mcp:
  caller_key: demo-app        # 工具挂载的调用方；留空或不配 mcp 段 = 不启用
  servers:
    - name: repo              # 字母/数字/_/-，≤32 字符，用于工具名前缀
      kind: repo              # stdio 适配器（可执行文件白名单，env 注入子进程）
      env:
        REPO_ROOT: 'C:/path/to/some/repo'
    - name: remote1           # Streamable HTTP 手写版（无鉴权，端点须为公网地址）
      kind: http
      endpoint: 'https://mcp.example.com/mcp'
      timeout_ms: 30000       # 可选，默认 30000
    - name: remote2           # 官方 MCP Go SDK 版（协议兼容性更强）
      kind: http_sdk
      endpoint: 'https://mcp2.example.com/mcp'
      timeout_ms: 30000
```

## 2. 代码仓库检索（`repo` 适配器）能力边界

### 2.1 数据源：仅本机目录

- 只能读**与本服务同机的本地文件系统目录**（`REPO_ROOT` 指向哪里就读哪里，不限于本项目）；
- **不能读远程仓库**：无 git clone、无 GitHub/GitLab API、无网络访问；
- 可配置**多个 server 实例**各指向一个本地目录，模型将看到 `repo_a_read_file`、`repo_b_read_file` 这样的分组工具；
- 想读远程仓库的临时办法：先手动 `git clone --depth 1` 到本地再配置指向。

### 2.2 工具集与读取上限

| 工具 | 行为 | 上限 |
|---|---|---|
| `list_files` | 列某目录一级条目 | 200 条（最大 1000） |
| `read_file` | 按行区间读单文件 | 默认 200 行、单次 ≤400 行差值；单文件 ≤2MB |
| `search_code` | 全仓大小写不敏感**包含匹配** | 50 条（最大 200）；跳过 >1MB 文件 |
| `find_symbol` | 等于 `search_code`（词法候选，**非语义/LSP**） | 同上 |
| `get_repo_map` | 全仓文件清单总览 | 300 个（最大 2000） |

明确**不做**：语义索引/引用查找（需 LSP/gopls）、正则/AST 级搜索、跨 server 的全局搜索、文件写入。

### 2.3 路径安全

- 相对路径统一在 `REPO_ROOT` 内解析，拒绝 `..` 穿越与绝对路径；
- 符号链接解析后必须仍在根内（防链接逃逸）；
- 忽略目录：`.git`、`.idea`、`.vscode`、`node_modules`、`vendor`、`dist`、`build`、`target`、`coverage`、`__pycache__`，以及除 `.github` 外的所有 `.` 开头条目。

## 3. 已评估、未实现的扩展路径

| 方向 | 说明 | 预估成本 |
|---|---|---|
| git 适配器（`kind: git`） | 启动时按 URL `git clone --depth 1` 到本地缓存（存在则 pull），复用 repotools；私有仓库走 token 环境变量 | ~100 行 |
| GitHub API 适配器 | `kind: github` 直调 contents/search API，不落本地盘；受 API 限速影响 | ~150 行 |
| HTTP 鉴权 | 按 server 配置静态凭证头（`Authorization` 等）；当前按需求实现为无鉴权 | ~30 行 |
| 会话复用 | HTTP 按次会话改「按 server 常驻会话 + 失效重建」；调用次数高频且 initialize 成为瓶颈时再考虑 | ~60 行 |

接入新 stdio 适配器的步骤：实现工具 → 在 `service/mcpclient` 的 `adapterWhitelist` 注册（可执行路径用字面量）→ 构建到 `bin/` → `custom.yaml` 增加一段 server 配置。

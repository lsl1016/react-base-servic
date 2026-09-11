# react-base-service（ReAct Agent 基座服务）

**可直接调用的 ReAct Agent 基座服务**：提供 ReAct 运行时及其完整周边能力（会话管理、历史回放、异步任务、工具产物、轮次反馈、附件），内置系统提示词装配、两段式工具加载（get_tool → execute_tool）、Skill 摘要注入与注册调用模式，开箱即用。

## 能力清单

| 能力 | 说明 |
|---|---|
| ReAct 运行时 | WebSocket 单入口 `/react-base-service/react/ws`，模型流式输出 + 工具循环 + 上下文自动压缩 + 模型互备 |
| 会话管理 | 会话隐式创建/复用（事务加锁 + 归属校验）、会话列表、并发 run 互斥、软删状态 |
| 历史回放 | 持久化消息还原为与实时协议同形的事件流（`/react/session/events`），内置回放页面 `/react/replay` |
| 工具体系 | Business Tool 注册（http/client 两类）、两段式加载、输入 Schema 校验、白名单、异步提交型工具 |
| Skill 体系 | Skill 注册 + 摘要索引注入 system 前缀 + get_skill 按需加载完整说明 |
| 系统提示词 | 按 callerKey + routeValues 前缀匹配解析，多条由通用到具体拼接 |
| 异步任务 | 提交快照落库、`<async_tasks>` 提醒注入、resolve/get 闭环工具、Provider 状态同步框架（可插拔） |
| 产物与附件 | python_exec 沙箱执行 + COS 产物下载；csv/md/txt 附件上传与引用 |
| 轮次反馈 | run 级点赞/点踩与问题反馈，会话维度回显 |
| 模型管理 | 用户自定义模型（modelHash 直引）、模型白名单、积分 |
| 内置页面 | playground 联调页（`/react/playground`）、回放页（`/react/replay`）、TypeScript SDK |

## 快速开始

```bash
# 1. 建库（MySQL）
mysql -h <host> -u root -p < sql/init.sql

# 2. 配置（编辑 conf/mount 下的四个 yaml，替换占位符）
vim conf/mount/resource.yaml   # MySQL / Redis / COS
vim conf/mount/api.yaml        # LLM 网关 / python-exec 沙箱
vim conf/mount/custom.yaml     # 模型目录、ReAct 运行时、服务 token

# 3. 构建前端 SDK（playground 页面依赖；不构建则仅 SDK 路由 404）
cd web/sdk && npm i && npm run build && cd ../..

# 4. 启动
go run main.go                 # 默认监听 :8080
```

详细文档：

- [系统架构](docs/architecture.md)
- [接入方式](docs/integration-guide.md)
- [启动与部署](docs/deployment.md)

## 目录结构

```
├── main.go                  # 入口：PreInit → InitResource → 路由 → 后台任务 → HTTP
├── router/                  # 路由注册（react / 注册类管理接口 / 静态资源）
├── controllers/http/        # HTTP/WS 控制器（react、attachment、caller、skill、tool、apikey、systemprompt、llmmodel）
├── service/react/           # ReAct 运行时核心（引擎、运行时装配、Meta Tool、会话历史、异步任务…）
├── service/asynctask/       # 异步任务 Provider 状态同步框架（可插拔）
├── service/tool|skill|...   # 工具/Skill/系统提示词/APIKey/Caller/模型积分 服务
├── api/llm/                 # LLM 客户端（claude / gpt 兼容 / minimax，流式+工具调用）
├── api/pythonexec/          # python_exec 沙箱客户端
├── models/llm/              # GORM 模型（17 张表，见表结构见 sql/init.sql）
├── components/              # 错误码、响应渲染、参数结构、路由前缀、COS、调用方运行时上下文
├── conf/                    # 配置加载 + conf/mount 示例配置（脱敏）
├── middleware/              # 免鉴权中间件（信任 X-User-Name 头，缺省 anonymous）
├── web/                     # playground / replay 页面 + TypeScript SDK（embed 进二进制）
├── sql/init.sql             # 全量建库脚本
└── docs/                    # 架构 / 接入 / 部署文档
```

## 范围与边界

- **只做通用 ReAct 基座**：提供与业务域解耦的 ReAct 运行时和注册管理接口，不内置知识库检索、Plan 编排等上层业务能力。
- **对外契约保持稳定**：ReAct 引擎主循环、系统提示词装配顺序（systemPrompt + 工具索引摘要 + Skill 索引摘要）、Meta Tool 集合与描述、两段式工具加载与指纹自愈、Skill 注入，以及注册类接口（caller/skill/tool/apikey/system-prompt）的请求响应结构。
- **通用化设计**：HTTP 工具请求头透传不绑定特定 caller；异步任务 Provider 框架默认无内置 Provider，按需注册扩展。

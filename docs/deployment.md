# 启动与部署

## 1. 环境要求

| 组件 | 要求 | 用途 |
|---|---|---|
| Go | ≥ 1.23（toolchain go1.23.11） | 编译 |
| MySQL | ≥ 5.7（建议 8.0，utf8mb4） | 全部业务表 |
| Redis | ≥ 5.x | 附件元数据缓存 |
| 腾讯云 COS | 一个私有桶 | 附件与 python_exec 产物存储 |
| LLM 网关 | claude / OpenAI 兼容 / minimax 端点 | 模型调用 |
| Python 沙箱 | python-exec 服务（可选*） | python_exec 工具 |
| Node | ≥ 20（仅构建 SDK 时） | web/sdk 构建 |

\* 不部署 python 沙箱时服务可正常启动与对话，仅 `python_exec` 工具不可用。
\* 全部 Go 依赖均为公共模块（gin v1.10 / gorm / redigo 等），无需访问内部 Git 源。

## 2. 建库

```bash
mysql -h <host> -u root -p < sql/init.sql
```

脚本幂等（`CREATE TABLE IF NOT EXISTS`），包含 17 张表的最终形态（已合并历史迁移）。库名默认 `llm`，与 `resource.yaml` 的 `mysql.llm.database` 对应，可按需修改。

## 3. 配置

配置文件位于 `conf/mount/`（golib 的 SubConfMount 目录，测试与本地运行均从此读取）。仓库内为脱敏示例，需替换 `REPLACE_WITH_*` 占位符。

| 文件 | 内容 | 关键项 |
|---|---|---|
| `config.yaml` | 框架基础 | `server.address`（默认 :8080）、日志级别 |
| `custom.yaml` | 业务配置 | `llm.models`（模型目录）、`llm.react.*`（运行时参数）、`async_task.enabled` |
| `api.yaml` | 外部 API | `llm.api_keys` + `llm.endpoints`（网关）、`python_exec` |
| `resource.yaml` | 存储连接 | `mysql.llm`、`redis.demo`、`cos.*` |

### custom.yaml 关键参数

```yaml
llm:
  react:
    max_steps: 8                    # 默认最大推理轮次
    stream_idle_timeout_sec: 120    # 模型流空闲超时（触发互备）
    models:                         # 前端可选模型与互备组
      default: {model_key: ..., model_version: ...}
      available: [...]
      mutual_failover: true
    context_compact:                # 上下文压缩
      token_trigger: 170000         # 高水位（也是入口 token 上限）
      token_target: 90000
    tool_result:                    # 工具大结果
      inline_limit_bytes: 262144
      db_ttl_sec: 604800
    playground_whitelist: []        # playground 白名单，空=不限制
```

## 4. 本地启动

```bash
# （可选）构建前端 SDK——playground 页面依赖，不构建仅 SDK 资源 404
cd web/sdk && npm i && npm run build && cd ../..

# 启动（默认 :8080）
go run main.go

# 验证
curl http://127.0.0.1:8080/react-base-service/react/playground   # playground 页面
```

启动流程：`helpers.PreInit`（应用名/配置/日志）→ `golib.Bootstraps`（recover 等）→ `helpers.InitResource`（Job/MySQL/Redis/COS）→ `router.Http`（路由）→ `router.Tasks`（异步任务同步框架）→ `http.Start`。MySQL/Redis 不可达会直接 panic，属预期行为。

## 5. Docker 构建

Dockerfile 三阶段：

1. `sdk-builder`：node 20 构建 `web/sdk` → `web/sdk/dist`；
2. `builder`：go 1.23 alpine，`go mod download` + 拷源码与 SDK 产物 → 编译二进制；
3. `go-runner`：仅携带二进制（静态资源已 embed）。

```bash
docker build -t react-base-service .
docker run -d -p 8080:8080 \
  -v $(pwd)/conf/mount:/apps/conf/mount \
  react-base-service
```

> 运行镜像内配置挂载路径以公司部署平台约定为准（golib 从应用相对路径 `conf/mount` 读取）。

## 6. 测试

```bash
go build ./...    # 编译
go vet ./...      # 静态检查（api/pythonexec 存在锁拷贝告警，非阻塞）
go test ./...     # 单测
```

已知环境依赖测试（本地无基础设施时失败）：
- `router` 包测试需要可用 MySQL；
- `service/token` 的分词估算测试依赖本地分词模型文件；
- `api/llm` 的 max_completion_tokens 用例依赖真实 `model_version_limits` 配置。

## 7. 运维要点

- **日志**：zlog 输出，run 级链路带 `runId/sessionId`；WS 断连/取消有结构化诊断日志（`[React.WS] diagnostic=`）。
- **优雅停机**：进程退出时活跃 run 统一置 `expired`（shutdown hook）。
- **超时兜底**：模型流空闲超时自动切互备模型；等待前端回填的 run 断连后置 `expired`。
- **容量**：单条工具大结果上限 16MB（MEDIUMTEXT + 业务限制）；附件单文件 50MB（存 COS）；产物单文件 10MB。
- **排障入口**：`tblLlmReactRun`（状态/token/快照）、`tblLlmReactMessage`（完整上下文与回放源）、`tblLlmReactAsyncTask`（异步任务）。

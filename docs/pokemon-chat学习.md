# pokemon-chat 学习笔记

> 调研对象：`C:\Users\keke\Desktop\xm\pokemon-chat`（「可萌」，FastAPI + LangGraph + Vue3，Apache 系开源）。
> 定位一句话：**一个教学级"全家桶"——把向量 RAG、知识图谱（Neo4j + LightRAG）、多智能体编排（LangGraph Supervisor+Workers）、高级 RAG 技巧（CRAG/Self-RAG/HyDE/查询分解）、MCP（既是 client 也是 server）、四套记忆、语音识别全部串进一个项目的完整参考实现。**
> 与本基座的关系：它恰好是《知识库与长期记忆集成改造方案.md》Layer2/3 的"自建路线"标本——看完会更清楚哪些设计值得抄、哪些是坑。

---

## 1. 总体架构

```
 Vue3 前端 (chat / agent / graph / KB工作台 / 地图 / 设置)
        │ NDJSON 流式 (POST /chat/ 或 /chat/agent/supervisor_agent)
        ▼
 FastAPI (server/, :5050，路由双挂载 / 与 /api)
        │
        ▼
 LangGraph: Supervisor ──Send() 并行──► rag / web / graph / stats / mcp worker ──► finalizer
        │                                        │
        │ 三级路由: 意图分类(高置信直连)            ├─ 向量: Milvus(IVF_FLAT) + bge-m3(1024维)
        │          → 规则关键词表                   ├─ 图谱: Neo4j + GraphCypherQAChain
        │          → LLM tool-calling 兜底          ├─ Web: Tavily
        │                                          └─ MCP: 自家 FastMCP(SSE, MySQL 坐标)
        └─ Pokédex 规则捷径（本地 JSON，0 次 LLM）
 基础设施(compose 分层 profiles): neo4j + bootstrap导入 / milvus(etcd+minio) / mysql / funasr
```

配置面三件套值得先记：

- **settings**（`src/core/settings.py`）：Pydantic v2 组合式 BaseSettings（env/.env），`get_settings()` 是 lru_cache 单例；默认栈：embedding=`BAAI/bge-m3`@siliconflow（1024 维）、reranker=`bge-reranker-v2-m3`、LLM=`Qwen2.5-7B-Instruct`@siliconflow。
- **feature flags 三优先级**（`src/core/feature_flags.py`）：`ui_config.json`（`/config` PATCH 持久化，**UI 不重启切能力**）→ env `enable_*`。开关粒度到 `enable_knowledge_base / knowledge_graph / web_search / mcp / reranker / self_rag / ner_bert / asr`。
- **LLM 工厂**（`src/core/llm_factory.py::build_chat_llm`）：唯一出口构建 `ChatOpenAI`（全走 OpenAI 兼容协议），凭据链 `ui_config.json → provider_secrets.json（/providers PATCH 原子写入）→ env`；支持 11 家 provider（siliconflow/openai/deepseek/zhipu/ark/qianfan/…），状态输出带脱敏。

## 2. 向量 RAG 栈是怎么接入的

| 环节 | 实现 | 要点 |
|---|---|---|
| Embedding | `src/models/embedding.py` 工厂（siliconflow/openai/ollama/dashscope） | **两级缓存**：内存 LRU(1万) → **SQLite 持久缓存**（`embedding_cache` 表，key=md5(model:text)，TTL 30 天）——重复语料零 embedding 成本 |
| 语义缓存 | `src/knowledge/cache/cache.py::SemanticCache` | 文件型（cache.json + embeddings.npy），按 `provider:model:api_base` 哈希隔离；余弦 **0.92** 命中、TTL 7 天；仅"无历史+无检索"的简单轮开启 |
| 向量库 | **Milvus**（两套封装并存） | `store/vector.py`（agent 路线，IVF_FLAT+COSINE，检索 top_k×2 → rerank 过滤）与 `store/knowledgebase.py`（工作台路线，IVF_FLAT+**L2**）——**度量不一致是坑** |
| 分块 | `core/indexing.py::chunk_file`(1000/100) 与 `ingestion/pipeline.py`(800/150) | RecursiveCharacterTextSplitter，**中文友好分隔符**（`\n\n → \n → 。！？；， → 空格`），Markdown 标题优先 |
| 邻块补全 | `VectorStore.get_adjacent_chunks(file_id, center_index, radius=1)` | metadata 存 chunk_index，命中块带邻居进上下文 |
| KB 元数据 | SQLite（`kb_db.py` + `models/kb_models.py`） | KnowledgeDatabase/File/Node 三表，chunk 级 CRUD 供前端管理 |

**工作台三步流**（知识库管理面的交互范式，最值得抄）：

```
POST /data/upload         文件落到 uploads
POST /data/file-to-chunk  解析+分块，返回 chunk 列表给前端【预览/编辑】(参数: chunk_size/overlap/do_ocr/use_deepdoc)
POST /data/add-by-chunks  前端确认(可能人工改过 chunk)后批量入库: batch_encode → Milvus insert → SQLite 记状态
```

把"分块结果可人工修订再入库"做成显式步骤，比 RAGFlow 黑盒解析更可控——对应基座改造方案 P3 管理面可以借鉴的交互。

## 3. 高级 RAG 技巧的接入位置（`src/graph/nodes/rag_worker.py`）

主检索链是一条完整的增强管线：

```
Self-RAG 判定要不要检索（enable_self_rag，默认关）
  → 查询分解 query_decomposer：启发式 is_complex("and/vs/compare/which") → LLM 拆 2-4 个子查询
  → 自适应 Top-K _adaptive_top_k：多子查询→8；query>100 字→6；否则 5
  → 条件 HyDE：复杂或长 query(>50字) 才生成假设文档
  → 并行子查询检索：ThreadPoolExecutor(min(n,4)) + as_completed
  → CRAG 纠错（enable_web_search 时）：LLM judge 三档 CORRECT/AMBIGUOUS/WRONG
       CORRECT→原文档；AMBIGUOUS→文档+Tavily 拼接；WRONG→仅 web；异常回退 AMBIGUOUS
  → 上下文拼装（score 降序+来源标注）→ LLM 生成
```

非 agent 路线（`src/knowledge/core/retriever.py::Retriever`）是 KB/Graph/Web/MCP 四路，**请求数 ≥2 即并行**（注释原话："max(step_latency) not sum"）。

接入完整度要分清（别学到半成品）：

| 技巧 | 状态 |
|---|---|
| CRAG（`nodes/crag.py`） | ✅ 完整接入 rag_worker |
| HyDE（`core/operators.py::HyDEOperator`） | ✅ 两处接入（rag_worker 条件触发 / rewrite_query 模式，默认 off） |
| Self-RAG（`core/self_rag.py`） | ⚠️ **只有 should_retrieve 挂进主管线**，relevance/support 幻觉核查没接 |
| Speculative RAG（`core/speculative_rag.py`） | ❌ 已实现但**零调用方**（孤立实验代码） |

## 4. 知识图谱的接入：两条路并存

**生产路：Neo4j + Cypher 问答**（`src/graph/nodes/graph_worker.py` + `src/knowledge/store/graph.py`）

- `Neo4jGraph` 包一层，`GraphCypherQAChain.from_llm` + **定制 few-shot Cypher 提示**（中文问→Cypher 四个示例）；
- 查不到回退本地 Pokédex 数据集；
- 图数据由 `scripts/import_graph.py` 一次性导入（compose 里 `neo4j-bootstrap` 服务，**marker 节点防重复导入**）；
- 可视化：`GraphSubgraphExtractor.get_subgraph(entity, hops)` 支撑前端 `/data/graph/node` 子图查询。

**实验路：LightRAG**（`src/knowledge/graphrag/lightrag_wrapper.py::PokemonLightRAG`）

```python
LightRAG(working_dir=…, workspace="pokemon_kb",
         llm_model_func=openai_complete_if_cache,       # OpenAI 兼容包装
         embedding_func=EmbeddingFunc(dim=1024, …),
         graph_storage="Neo4JStorage",       # 图进 Neo4j
         vector_storage="MilvusVectorDBStorage",  # 向量复用 Milvus
         kv_storage="JsonKVStorage")         # KV/状态走本地 JSON
```

查询固定 `mode="mix" + only_need_context=True`（只要上下文不要答案，答案仍由主 LLM 生成）——这是 LightRAG 作为"检索器"而非"问答器"的正确用法。**但它只挂在旧版 chat_agent 的 graph_rager 节点，新 supervisor 拓扑未使用，前端也没有 ingestion 入口**（graphrag_raw_data 指向一本小说文本，离线实验素材）。

## 5. LangGraph 编排：Supervisor + 5 Workers（`src/graph/workflow.py`）

**三级路由**是这套图的核心设计（`nodes/supervisor.py`）：

1. `classify_intent` → 意图是 POKEDEX_FACTS 且置信度 ≥0.8 → 直连对应 worker（`forward_directly=True`，**确定性答案 0 次 LLM 路由**）；
2. 规则关键词表（`nodes/rule_router.py`：中文关键词→worker，ASCII 词做边界正则，年份正则→web）——多命中返回 `__PARALLEL__`；
3. 兜底 LLM tool-calling 路由：动态 handoff tools（`route_to_{worker}/finish/route_parallel`），`tool_choice="required"`，temperature=0。

**并行执行**用 LangGraph `Send` API：supervisor 路由返回 `[Send(worker, state) for …]`，各分支独立跑完由 finalizer 聚合多份 AIMessage。**finalizer 本身是个"润色器"**（中文系统提示：只改写不新增事实、不泄露内部链路），可用 feature flag 关掉。

周边还有一整套**中间件链**（`src/agents/middleware/`）：`wrap_model_call` 责任链模式挂 logging / memory（trim 或 summarize，窗口 50 条）/ retry（指数退避）/ fallback（多模型降级）/ injection / long_term_memory——"用中间件包装 LLM 调用"的干净示范（`RunnableLambda(middleware.wrap_model_call(base_llm.invoke, ctx))`）。

另有一套旧版图（`src/agents/chat_agent.py`：guardrail→intent_router→supervisor 循环→7 个领域 agent）和 deep_agent（open-deep-research 式迭代研究环）——**deep_agent 节点逻辑目前是硬编码规则模拟，LLM 未接入**，实验性。

## 6. MCP：既是 server 也是 client

- **Server**（`src/mcp/mcp_server.py`）：`FastMCP("pokemon-fastmcp")` SSE 传输（:8000），两个工具查 MySQL 坐标表（`search_locations_by_pokemon` / `get_location_info`），DBUtils 连接池；compose 里独立 `mcp` service。
- **Client**（`src/mcp/client_core.py::MCPClient.ask`）：经典三段式——`sse_client + ClientSession`（**每次新建会话**）→ `list_tools` 转 OpenAI function tools → 本地 chat completion（tool_choice=auto）→ `session.call_tool` 执行 → **二次 completion 总结工具结果**。LLM 配置可独立于主链（`MCP_LLM_*`）。

对比基座：这套 client 是"无注册表、每次握手、按调用重建"的简化版；基座 MCP 体系（注册表同步 + 两段式加载 + 动态连接管理）成熟得多，pokemon-chat 的参考价值在 server 侧——**"把业务数据库包成 FastMCP 工具"是半天工作量的事**，基座将来想让 Agent 查内部库可以照抄。

## 7. 记忆：四套并存（反面教材）

| 记忆 | 存储 | 接入 |
|---|---|---|
| 会话历史 | 内存 | `core/history_chat.py`，system prompt 服务端管理 |
| Agentic 偏好记忆 | SQLite（conversations+preferences） | 每 5 轮 / 20% 概率触发抽取，偏好注入 system prompt |
| 语义长期记忆 | **Chroma** | 经 `LongTermMemoryMiddleware` 挂进 chat_agent；**写死 text-embedding-3-small**（与主栈 bge-m3 不一致，坑） |
| **Mem0** | Qdrant（本地） | `server/routers/memory_router.py` 独立 CRUD，与前三个零复用 |

四套互不复用、嵌入模型不一致——**印证了基座"三层分工 + 统一写核心"设计的必要性**：记忆体系宁少勿乱。

## 8. 文档摄取与部署

**解析器路由**（`src/knowledge/ingestion/parsers/base.py::parse_file`）：

```
use_deepdoc 强制 → DeepDoc（pdf/docx/ppt/xls 布局识别视觉栈）
PDF + do_ocr     → RapidOCR（不可用回退 DeepDoc）
PDF 默认         → PyPDFLoader + 乱码检测（CID 标记/有效字符占比<0.5/Latin-Extended>20% → 转 OCR）
PPT/PPTX         → DeepDoc
DOCX/XLS/CSV/TXT/MD → MarkItDown（失败回退 DeepDoc）
```

**增量刷新**（`core/refresh.py`）：web/directory 定向刷新，内容 hash 变更检测 + 版本记录 + SQLite refresh log，`/refresh/bulbapedia` 预置 URL。

**部署**（`docker/docker-compose.yml`）分层 profiles：默认 `api(5050)+web(nginx 3100)`；`infra` 加 neo4j:5.14（`NEO4J_AUTH=none`）+ bootstrap 导入 + Milvus 全家桶（etcd+minio+milvus:2.3）+ mysql:5.7；`mcp` / `asr` 各自独立 profile。Makefile 对应 `docker-up / docker-up-infra / docker-up-full`。`.env.example` 逐项中文注释，是配置面最佳阅读材料。安全上仅适合教学：mysql root 密码硬编码、Neo4j 无认证。

## 9. 半成品清单（学习时绕行）

1. `stats_worker.analyze()` 是 TODO 桩——stats 实际只靠属性克制表；
2. Self-RAG 只接了 1/3（should_retrieve）；
3. Speculative RAG 零调用方；
4. deep_agent 节点是规则模拟，注释自认"实际应用中由 LLM 生成"；
5. LightRAG 不在新主拓扑里、无前端入口；
6. `tool_router.py` 的 agent 流式子路由整段注释、`/chat/models/update` 返回 501；
7. 两套 Milvus 封装度量不一致（COSINE vs L2）、前端后端各一份分块逻辑。

## 10. 对本基座的启示（对照改造方案）

1. **自建 RAG 的真实成本**：pokemon-chat 用整个 `src/knowledge/`（几十个文件）自建了 Milvus+分块+解析+缓存+检索增强链——这是"不部署 RAGFlow"路线的价格标签。反过来说明基座方案"**RAGFlow 成品引擎 + 薄 Go 客户端**"的性价比：CRAG/查询分解这些客户端技巧将来可选加，但解析/索引/检索底座不必自养。
2. **embedding 两级缓存（LRU→SQLite）**：若基座将来做任何自建嵌入（memory P4 或注入块向量化），照抄这个设计，重复语料零成本。
3. **chunk 工作台三步流**（upload→预览编辑→确认入库）：比黑盒解析可控，值得作为基座知识管理面 P3 的交互蓝本。
4. **三级路由 + 本地确定性捷径**：规则优先、LLM 兜底的路由分层，和"Pokédex 本地数据集直接出答案（0 次 LLM）"的确定性捷径，对基座高频问题场景（常见 SOP 直接命中）有借鉴；基座的三层记忆分流协议（改造方案 §5.1）本质是同一思想的模型侧版本。
5. **finalizer 润色器**：多 worker 并行产出后的聚合/润色节点，对基座并行委派（`subagent.max_parallel>1`）之后的答案收敛是个可行补充。
6. **四套记忆并存的混乱**是反面教材，基座"Layer1 统一写核心 + Layer2 episode 异步 + Layer3 文档"的分层纪律要守住。
7. **feature flags 三优先级**（UI 热切换→env）：基座目前是 yaml 重启生效，若知识/图谱能力要灰度，`/config` PATCH + json 持久化的模式可以参考。

## 11. 提取清单（与改造方案的落地对照）

按"基座已有 / 方案已覆盖 / 值得提取"三个筛子过的最终结论：**从这个项目提取交互设计与缓存/检索增强的设计，不提取任何基础设施**——基础设施层它要么不如基座（MCP client、编排、记忆），要么已被 RAGFlow/Graphiti 方案覆盖（解析、向量库、图谱引擎）。

### 值得提取（3 项）

| # | 提取物 | 形态 | 落点 |
|---|---|---|---|
| 1 | **chunk 工作台三步流**（upload → 预览/人工编辑分块 → 确认入库） | 交互设计 | 改造方案 §3.5（P3 管理面，后端对接 RAGFlow 文档/chunk API 实现同样交互，前端参考 `DatabaseRagWorkbench`） |
| 2 | **embedding 两级缓存**（内存 LRU → SQLite 持久缓存，md5(model:text) 做 key，TTL 30 天，约 150 行 Go） | 设计 | 条件触发：memory P4 / 任何自建嵌入的第一块砖，P1-P3 不做（嵌入分别归 RAGFlow 与 Graphiti） |
| 3 | **客户端检索增强链**（查询分解 + 自适应 Top-K 必选；CRAG/条件 HyDE 可选） | 设计 | 改造方案 §5.3（P4 可选：P1 先裸检索，观测 badcase 后逐项开启） |

### 可选参考（4 项，暂不排期）

| # | 参考点 | 原实现位置 | 何时考虑 |
|---|---|---|---|
| 1 | feature flags 三优先级热切换（`/config` PATCH → `ui_config.json` 持久化 → env） | `src/core/feature_flags.py` | 知识/图谱能力需要不重启灰度切换时；落地前先理清与基座 yaml conf 体系的关系，避免双轨 |
| 2 | 增量刷新管线（内容 hash 变更检测 + 版本记录 + SQLite refresh log） | `src/knowledge/core/refresh.py::KnowledgeRefreshManager` | 内部文档目录/网页源要定时同步进 RAGFlow 时（RAGFlow 自身的增量能力先摸底，重复就不做） |
| 3 | 图谱子图可视化 API（按实体抽 N 跳子图 + 前端去重渲染） | `GraphSubgraphExtractor.get_subgraph` + `/data/graph/node` | P4 运营面要可视化 Graphiti 数据时（Graphiti 侧需经 MCP 的 `get_episode_entities` 或自写 Cypher 补齐） |
| 4 | "内部数据库包成 FastMCP server"（两个工具 + 连接池，约半天工作量） | `src/mcp/mcp_server.py` | Agent 需要查内部业务库时——基座 MCP 体系现成（连接管理/工具同步/两段式加载全有），只需照抄 server 侧写法 |

### 明确不搬

- **MCP client**：每次握手、无注册表的简化版，基座现有体系（注册表同步 + 动态连接 + SSRF 校验）强得多；
- **LangGraph 编排**：delegate_agent + 隔离子 run 已覆盖同能力，且 Python 框架进不了 Go 基座；
- **四套记忆**（会话历史/Agentic 偏好/Chroma 语义/Mem0 互不复用、嵌入模型不一致）：反面教材，基座"三层分工 + 统一写核心"纪律不破；
- **文档解析栈**（DeepDoc/MarkItDown/乱码检测/OCR 路由）：RAGFlow 内置的 DeepDoc 本就是同源技术，方案里已归 RAGFlow；
- **多 provider LLM 工厂**：基座的模型解析 + 互备已覆盖；
- **语义缓存**（余弦 0.92 直接返回）：知识库更新后的失效问题没解好，它自己也只敢对"无历史无检索"的简单轮开启，收益太小。

# Graphiti 学习笔记

> 调研对象：`C:\Users\keke\Desktop\xm\graphiti`（getzep/graphiti，包名 `graphiti-core` **v0.30.2**，Python ≥3.10，Apache-2.0）。
> 定位一句话：**把"发生过的事"（Episode）增量抽取成带有效期、可溯源、可混合检索的时间性知识图（Temporal Context Graph），作为 Agent 的长期记忆/上下文引擎。** 它是 Zep 的开源核心（论文 arXiv:2501.13956）；Zep 是托管服务，Graphiti 是自带图数据库的框架。
> 与本基座的关系：补齐 MySQL 条目式记忆（`docs/memory.md`）缺失的"实体关系 + 时间演化 + 混合检索"层，接入方式见《知识库与长期记忆集成改造方案.md》。

---

## 1. 心智模型：与传统 RAG 的本质差别

```
传统 RAG：  文档 → chunk → embedding → 向量库 → 相似度检索
Graphiti：  Episode → LLM抽取 → Entity ──Fact(边)──> Entity → 时间性知识图
                                                    ↓
                     Vector + BM25 + Graph BFS + 时间过滤 → 混合检索
```

三条设计支柱（README "What is a Context Graph"）：

1. **事实带有效期**（validity window）：新事实不是覆盖旧事实，而是让旧事实"失效"并保留历史；
2. **实体带演化摘要**（summary 随周边事实更新）；
3. **一切可溯源**：每条 Fact 记录来源 Episode uuid，可回到原始事件。

## 2. 数据模型（`graphiti_core/nodes.py`、`edges.py`）

### 2.1 五类节点

| 节点 | 字段要点 | 说明 |
|---|---|---|
| `EpisodicNode` | `source`、`content`、`source_description`、**`valid_at`（事件时间）**、继承 `created_at`（摄取时间）、`entity_edges[]`、`episode_metadata` | "发生过的一次事"；episode 本身也是图节点 |
| `EntityNode` | `name`、`name_embedding`、`summary`、`attributes`（按本体 schema 提取）、`labels[]` | 长期稳定的"东西"；summary 是周边事实的区域摘要 |
| `CommunityNode` | `name_embedding`、`summary` | 关系密切实体簇（标签传播聚类 + LLM 摘要） |
| `SagaNode`（0.30 新增） | `summary`、first/last_episode_uuid、双水位线（`last_summarized_at` 墙钟 / `last_summarized_episode_valid_at` 事件时间） | 长故事线的**增量摘要**单元 |

`EpisodeType` 有 **4 种**（常见教程只说 3 种）：`message`（格式必须是 `"actor: content"`）、`json`、`text`、`fact_triple`（0.30 新增）。

### 2.2 六类边——知识真正存放在**边**上

| 边 | 关系类型 | 说明 |
|---|---|---|
| `EpisodicEdge` | `:MENTIONS` | Episode → Entity（溯源链） |
| **`EntityEdge`** | `:RELATES_TO` | **核心：一条 Fact** |
| `CommunityEdge` | `:HAS_MEMBER` | Community → Entity |
| `HasEpisodeEdge` | `:HAS_EPISODE` | Saga → Episode（0.30） |
| `NextEpisodeEdge` | `:NEXT_EPISODE` | Episode 按时间成链（0.30） |

`EntityEdge` 的关键字段：

```
name（关系名）          fact（自然语言事实句）      fact_embedding
source/target_node_uuid  episodes[]（来源溯源）
created_at（系统写入时间） reference_time（episode 参考时间）
valid_at（事实开始为真）  invalid_at（事实停止为真）  expired_at（被新事实取代的时刻）
attributes（按本体 schema 提取的边属性）
```

### 2.3 双时间轴（bi-temporal）——最重要的语义

```
valid_at / invalid_at / reference_time   ← 业务世界时间（事件什么时候发生/失效）
created_at / expired_at                  ← 系统时间（Graphiti 什么时候写入/淘汰）
```

今天补录一条上个月发生的事：`created_at` 是今天，`valid_at` 是上个月。查询"现在"与查询"上个月"会得到不同答案。

## 3. 摄取管道：`add_episode()` 逐步拆解（`graphiti.py` L1043-1288）

这是源码阅读的第一主线。精确步骤：

```
add_episode(name, episode_body, source_description, reference_time,
            source, group_id, entity_types, edge_types, edge_type_map,
            custom_extraction_instructions, previous_episode_uuids,
            update_communities, saga, ...)
  │
  1  validate + _resolve_request_scope(group_id)   ← 按 group clone driver/clients（修复 #1676 并发错库）
  2  取上文：previous_episode_uuids 未给则 retrieve_episodes(last_n=10)
  3  创建 EpisodicNode（valid_at=reference_time, created_at=now）
  4  默认 edge_type_map = {('Entity','Entity'): [...]}
  5  extract_nodes 【LLM①】按 source 选 prompt；先内存合并完全重名
  6  resolve_extracted_nodes（去重）
  │    ├ 每个抽取名生成 embedding → 余弦检索候选（15 个、阈值 0.6）
  │    ├ 确定性：归一化名精确匹配
  │    └ 升级 LLM 去重【LLM②】（duplicate_candidate_id，-1=新建）
  7  extract_edges【LLM③】（max_tokens=16384；校验端点必须在节点表、丢自环、解析时间串）
  │  resolve_extracted_edges：
  │    ├ 内存精确去重（source+target+归一化 fact）
  │    ├ 每条边取同端点现有边 + 两次 EDGE_HYBRID_SEARCH_RRF 检索（重复候选 / 失效候选）
  │    ├ 快速路径：fact 与同端点现有边完全一致 → 复用边、追加 episode uuid（无 LLM）
  │    ├ 否则 LLM 判重/判矛盾【LLM④】
  │    ├ 无候选时轻量补时间【LLM⑤ small 模型】+ 自定义属性提取【LLM⑥】
  │    └ resolve_edge_contradictions：纯时间比较——旧边 valid_at < 新边 →
  │        candidate.invalid_at = 新边.valid_at；expired_at = now   ★失效而非删除
  8  extract_attributes_from_nodes【LLM⑦】（overlay 合并保旧值）+ 实体摘要（短则拼 fact 无 LLM，长则批量【LLM⑧】）
  9  _process_episode_data：MENTIONS 边 + bulk 落库 + Saga 链（NEXT_EPISODE / HAS_EPISODE）
 10  update_communities=true 时并行更新社区摘要【LLM⑨，可选】
```

一次典型 ingest 的 LLM 调用量：**最少 3-4 次**（抽取节点/边 + 判重），属性/时间/摘要按需增加——这决定了写入成本与延迟（数秒级），**写入必须异步化**，不能挡在对话主链路上。

其余写入口：

- `add_episode_bulk`：批量合并抽取 + 内存去重 + 对图解析，一次落库；
- `add_triplet`：直接写三元组（绕过抽取，但仍做实体解析与判重/失效）；
- `graphiti.nodes.entity.save()` 等 **namespaces API**：直接 CRUD，不走 LLM（管理面/回灌用）。

## 4. 搜索体系（`graphiti_core/search/`）

### 4.1 四层对象 + 三路方法

| 层 | 可用方法 | 可用 reranker |
|---|---|---|
| edge（Fact） | cosine / bm25 / bfs | rrf / mmr / node_distance / episode_mentions / cross_encoder |
| node（Entity） | cosine / bm25 / bfs | 同上 |
| episode | 仅 bm25 | rrf / mmr / cross_encoder |
| community | cosine + bm25 | rrf / mmr / cross_encoder |

`SearchConfig` 四个子配置（None=不搜该层）+ `limit`(10) + `reranker_min_score`(0)；`sim_min_score` 默认 0.6，`bfs_max_depth` 默认 3。

### 4.2 执行流（`search.py`）

```
查询 → （需要则）embed 查询向量 → 四层并行（semaphore_gather）
  edge_search：每路取 2×limit 候选
    ├ edge_fulltext_search   BM25（Neo4j db.index.fulltext.queryRelationships）
    ├ edge_similarity_search 向量（vector.similarity.cosine）
    └ edge_bfs_search        图扩展（启用了 bfs 且无 origin 时，用前几路结果的
                              source 节点做一次追加扩展——三路联动的设计亮点）
  → rerank：
      rrf:            score += 1/(rank+1)          ← Reciprocal Rank Fusion
      cross_encoder:  先 rrf 种子取 2×limit → rank() → min_score 过滤
      node_distance:  需 center_node_uuid，先 rrf 再按图距离（1 跳=1 分）
      episode_mentions: 按 edge.episodes 数量降序（被越多来源提及越可信）
```

入口两级：`search()`（基础：无 center 用 `EDGE_HYBRID_SEARCH_RRF`，有 center 用 `NODE_DISTANCE`）；`search_()`（高级：默认 `COMBINED_HYBRID_SEARCH_CROSS_ENCODER`，四层全开，返回带分数的 SearchResults）。

### 4.3 过滤器（`search_filters.py`）

`SearchFilters`：node_labels、edge_types、`valid_at/invalid_at/created_at/expired_at`（外 OR 内 AND、7 种比较符含 IS NULL）、property_filters（属性名+值+操作符）——**时间过滤是检索的一等公民**，"上个月 service-A 部署在哪"就是一次 `invalid_at IS NULL` 与时间窗组合查询。

## 5. LLM / Embedder / Reranker 抽象

| 组件 | 抽象 | 实现 |
|---|---|---|
| LLM | `LLMClient.generate_response(messages, response_model, model_size, ...)` | OpenAI（默认 **gpt-5.5** / small **gpt-4.1-nano**）、Azure、Anthropic、Gemini、Groq、**OpenAIGenericClient（任意 OpenAI 兼容端点：Ollama/vLLM/OpenRouter）**、GLiNER2（本地抽取，非生成） |
| Embedder | `EmbedderClient.create/create_batch` | OpenAI（默认 text-embedding-3-small）、Azure、Gemini、Voyage；维度 `EMBEDDING_DIM` 默认 1024 |
| Reranker | `CrossEncoderClient.rank(query, passages)` | OpenAIRerankerClient（**LLM 伪交叉编码器**：gpt-4.1-nano 每 passage 一次布尔分类、logit_bias 锁 True/False、top logprob 当分数）、BGERerankerClient（本地 bge-reranker-v2-m3）、Gemini |

基类内建能力值得学：Pydantic schema 序列化注入末条消息、**tenacity 重试**（4 次、5-120s 指数退避，针对 429/5xx/空响应/JSON 解析失败）、TokenUsageTracker、属性抽取防泄漏 preamble（防模型把字段描述抄进值）、small/medium 双模型分工（去重/时间/属性用小模型）。并发用 `SEMAPHORE_LIMIT`（默认 20）信号量包 gather，防 429。

> 对基座的意义：`OpenAIGenericClient` + 自建 embedder 端点意味着 Graphiti 的模型供应商可以完全复用基座已有的 OpenAI 兼容网关，不新增外部依赖。

## 6. 图数据库驱动（`graphiti_core/driver/`）

支持 **Neo4j / FalkorDB / Kuzu（已标弃用）/ Neptune(+OpenSearch)**。架构是迁移中的 IoC：`GraphDriver` 挂 `search_interface` / `graph_operations_interface`，模型方法优先走接口、`NotImplementedError` 回退内联 Cypher；每个后端有 operations/ 子包。`driver.clone(database=group_id)` 支持按 group 映射独立数据库。`build_indices_and_constraints` 建 4 个全文索引（episode_content、node_name_and_summary、community_name、edge_name_and_fact）+ 范围索引。

> 选型：轻量起步选 **FalkorDB**（Redis 模块，单容器；`falkordblite` 可内嵌）；企业存量选 Neo4j。

## 7. 本体定制（Ontology）——运维场景的关键能力

- `entity_types: dict[str, type[BaseModel]]`：类型名 → Pydantic 模型，**docstring 即类型描述**（注入抽取 prompt），模型字段被抽成节点 `attributes`；字段名不得与内置字段冲突（否则 `EntityTypeValidationError`）。
- `edge_types` + `edge_type_map: dict[(源类型,目标类型), 允许的边类型列表]`：约束"哪类节点之间允许什么关系"。
- `excluded_entity_types`、`custom_extraction_instructions`（附加自然语言抽取指令）。
- MCP server 支持从 **YAML 配置**注册本体（`mcp_server/src/models/{entity_types,edge_types}.py`）——不用改 Python 代码就能换 schema。

基座运维场景的初始本体（Service/Cluster/Job/Table/Incident/Error/Solution + DEPENDS_ON/READS_FROM/RESOLVED_BY 等）已在草稿讨论中成型，落地即写两份 Pydantic/YAML。

## 8. group_id 多租户

- 每个节点/边带 `group_id`（图分区），所有检索/抽取/判重查询都追加 `group_id IN $group_ids`；
- 校验 `^[a-zA-Z0-9_-]+$`；可进一步映射为**独立数据库**（Neo4j database / FalkorDB graph key）；
- MCP server 的 QueueService **按 group_id 串行**处理 episode 队列（同组避免写竞态，跨组并行）。

> 对应基座的映射：`group_id ≈ callerKey|userName`（或先只到 caller 级），与记忆模块 `owner_type/owner_key` 的两级作用域同构。

## 9. 服务化形态（两条现成的暴露通道）

### 9.1 FastAPI REST（`server/graph_service/`，端口 8000）

| 路由 | 说明 |
|---|---|
| `POST /messages` | **202 异步**：入 AsyncWorker 队列后台 add_episode（写路径天然异步，与 §3 的成本结论一致） |
| `POST /entity-node` | 直接建实体（201） |
| `DELETE /entity-edge/{uuid}`、`/episode/{uuid}`、`/group/{group_id}`、`POST /clear` | 删除/清库 |
| `POST /search` | 查 facts |
| `GET /episodes/{group_id}?last_n=` | 拉原始 episode |
| `POST /get-memory` | 多条消息拼成查询取 facts（"给 Agent 取记忆"的便捷封装） |
| `GET /healthcheck` | 健康检查 |

### 9.2 MCP server（`mcp_server/`，`--transport http` 默认，端点 `/mcp/`，端口 8000）

工具集：`add_memory`（异步队列、支持本体/自定义指令/saga 参数）、`search_nodes`、`search_memory_facts`（支持 edge_types + 四个时间边界过滤）、`add_triplet`、`delete_entity_edge`、`delete_episode`（只删该 episode 独有的实体/事实）、`get_entity_edge`、`get_episodes`、`get_episode_entities`（溯源）、`summarize_saga`、`build_communities`、`clear_graph`、`get_status`；另有 `/health`。

CLI 关键参数：`--llm-provider --embedder-provider --database-provider --group-id --config(YAML)`；HTTP 传输即可被基座现有 MCP 客户端（`kind: http` / `http_sdk`，带 SSRF 校验与 `allow_private_endpoint` 开关）直连，工具自动同步进 `tblLlmTool`。

## 10. 维护工具

- `build_communities(group_ids)`：标签传播聚类（内置实现，非 GDS 依赖）→ 每簇 LLM 摘要 → CommunityNode + HAS_MEMBER；
- `summarize_saga(saga_id)`：**增量摘要**——只处理 `created_at > last_summarized_at` 的 episode（≤200），旧摘要进上下文；双水位线防回退；
- `remove_episode`：只删该 episode 首创的边和仅被它 MENTIONS 的节点（精准回滚单条来源）；
- `clear_data(driver, group_ids)`：全删或按组删；
- 可观测：OTel tracing（默认 NoOp）+ PostHog 遥测（可关）。

## 11. 源码阅读顺序（按依赖关系）

```
① 数据模型        graphiti_core/nodes.py → edges.py
② 总入口          graphiti_core/graphiti.py :: add_episode()
③ 实体构建        utils/maintenance/node_operations.py（extract_nodes / resolve_extracted_nodes）
④ Fact 构建       utils/maintenance/edge_operations.py（extract_edges / resolve_extracted_edge /
                  resolve_edge_contradictions ★时间失效的核心）
⑤ 检索            search/search_config_recipes.py → search/search.py → search/search_utils.py
⑥ 存储驱动        driver/driver.py → neo4j_driver.py（或 falkordb）
⑦ 服务化          server/graph_service/ → mcp_server/src/graphiti_mcp_server.py
⑧ 横切            llm_client/client.py（重试/schema 注入）、prompts/（全部 prompt 模板）
```

回答草稿里悬着的三个问题（源码结论）：

1. **LLM 到底调几次**：最少 3-4 次（抽实体、抽边、节点去重、边判重），属性/时间戳/摘要/社区按需再加；small 模型承担轻活。
2. **怎么判重复/冲突**：候选靠 embedding 余弦（阈值 0.6、15 候选）+ 两次混合检索（重复候选限定 valid 边、失效候选全组），判定靠 LLM 结构化输出（duplicate_facts + contradicted_facts 索引）。
3. **旧记忆如何更新而非覆盖**：`resolve_edge_contradictions` 纯时间比较——旧边 `invalid_at = 新边.valid_at`、`expired_at = now`，历史保留可查；fact 完全一致时走无 LLM 快速路径复用边并追加溯源。

## 12. 与 Letta / Mem0 / RAGFlow 的边界（一句话版）

| | Graphiti | Letta | Mem0 | RAGFlow |
|---|---|---|---|---|
| 本质 | 记忆/知识**检索引擎**（无 Runtime） | 带 Memory 的完整 **Agent Runtime** | Memory Layer（事实抽取/检索） | 企业**知识库**/RAG 引擎 |
| 记忆结构 | Episode→Entity→Fact（图+时间） | Memory Blocks（进上下文的持久区） | 扁平事实条目 | 文档→chunk→索引 |
| 时间语义 | ★bi-temporal，事实失效不删除 | 无专门模型 | 有时间召回，无冲突失效 | 无 |
| 检索 | 向量+BM25+BFS+时间过滤+rerank | Recall/Blocks | 语义+BM25+实体 | Hybrid RAG+Graph/Tree |

组合结论（与 `docs/memory.md` 的既有取舍一致）：**基座已有 Letta 式"常驻核心记忆"（MySQL 条目层）＋ ReAct Runtime；Graphiti 补"时序事实图谱层"；RAGFlow 补"文档知识库层"——三层各司其职，具体见改造方案。**

## 13. 已知工程注意点

- 写入延迟秒级、LLM 成本按 episode 计——写入必须异步（官方 server 也是 202 队列化），且要给"哪些内容值得入图"立规矩（不是所有对话都进）；
- 0.30.2 相对常见教程的差异：EpisodeType 多了 `fact_triple`；Saga/双水位线；driver IoC 重构；`search_()` 取代 `_search()`；请求级 driver clone；
- Kuzu 驱动已标弃用，新部署不要选；
- 默认 reranker 是 OpenAI LLM 伪交叉编码器（非 OpenAI 部署要用 BGE/Gemini 替换，factories 已处理回退问题）；
- 遥测默认开（PostHog），内网部署记得关。

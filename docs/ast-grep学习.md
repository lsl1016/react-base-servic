# ast-grep 学习笔记

> 调研对象：[ast-grep/ast-grep](https://github.com/ast-grep/ast-grep)（Rust，MIT，作者 HerringtonDarkholme），文档站 <https://ast-grep.github.io/>。
> 定位一句话：**用"代码本身"当搜索模式的结构化 grep**——官方自称 "a hybrid of grep, eslint and codemod"（grep 的用法 + eslint 的规则体系 + codemod 的重写能力），底层用 Tree-sitter 把源码解析成语法树后做结构匹配。
> 与本基座的关系：`repotools/pattern.go` 的 `search_pattern` 工具（`repo_search_pattern` / `ws_<service>_search_pattern`）就是它的 Go-only 简化移植；本文记录它的完整设计，以及我们吸收了什么、刻意没吸收什么。

---

## 1. 心智模型：为什么"用代码搜代码"比正则强

文本搜索（grep/ripgrep）的三个盲区，ast-grep 全部解决：

| 场景 | 文本/正则 | ast-grep |
|---|---|---|
| 字符串/注释里出现同样代码字面量 | 误命中 | 不命中（树上根本没有这个节点） |
| 跨行、空白、缩进差异 | 需要人肉写 `\s*` | 天然不敏感（比较的是树不是字符） |
| "这种结构的所有位置"（任意嵌套深度） | 极难表达 | pattern 自动在**每个节点**上尝试 |

一句话：**正则作用于字符流，ast-grep 作用于语法树**。pattern `console.log($$$)` 既能命中顶层调用，也能命中嵌在赋值、参数、回调里的调用——因为遍历的是树的所有节点。

## 2. 核心机制：pattern 如何变成匹配器

```
pattern 源码（含元变量）
   ↓ Tree-sitter 解析（多语言：Go/Java/Python/TS/Rust/C++…20+）
带占位节点的语法树
   ↓ 遍历目标代码的语法树，逐节点尝试结构匹配
命中节点 + 元变量绑定
```

关键点：**pattern 本身必须是可解析的合法代码**。元变量不是词法魔法，是先替换成占位符再解析（我们的 Go 移植同样如此——`$NAME` 不是合法 Go 标识符，先换成 `__sgOne0__` 这类占位 ident 再交给 `go/parser`）。解析有歧义时（如 JS 的 `a` 既是表达式又是语句），用对象式 pattern 补上下文：

```yaml
pattern:
  context: class { $F }     # 先解析完整上下文
  selector: field_definition # 再选中其中关心的节点
```

## 3. Pattern 语法：元变量语义（我们移植的部分）

| 写法 | 语义 | 类比 |
|---|---|---|
| `$VAR` | 匹配**任意单个节点**（任何类型位置） | 正则的 `.`，但作用于树 |
| `$$$VAR` | 匹配**零或多节点**，仅列表上下文（实参/形参/语句块/字段） | 正则的 `*` |
| 同名 `$A ... $A` | 两处必须匹配**文本相同**的代码 | 正则捕获组回引 `\1` |
| `$_FOO`（`_` 前缀） | 匿名：不捕获、多处可不同 | 非捕获组 `(?:...)` |
| `$$VAR`（双美元） | 匹配 Tree-sitter 的 unnamed 节点（Go/ast 无此概念） | — |

元变量命名规则：大写字母/`_`/数字（`$META`、`$_123` 合法；`$invalid`、`$kebab-case` 非法）。

三个语义细节值得记住（都体现在我们的测试里）：

1. **同名一致性比较的是源码文本**，不是节点身份——`$A == $A` 匹配 `a == a` 不匹配 `a == b`；
2. `$$$` 是"零或多"：`fmt.Errorf($FMT, $$$ARGS)` 也命中无参形式；
3. 多语句 pattern 匹配的是**连续语句序列**（块内每个起点尝试），单语句 pattern 等价于"匹配每个该类语句节点"。

## 4. 规则体系：从 pattern 到 eslint（我们没移植的部分）

pattern 只是最低层。完整的 rule object 体系是 ast-grep 的"eslint 面"：

### 4.1 Atomic Rules（原子规则）

- `pattern`：如上；
- `kind`：按节点类型匹配（Tree-sitter 的 node kind，如 `call_expression`），0.39+ 支持 ESQuery 风格 `kind: call_expression > identifier`；
- `regex`：Rust 正则，须匹配节点**全文**（无 look-around/回引）；
- `nthChild`：按兄弟序数（1-based，同 CSS，支持 An+B）；
- `range`：按行列窗口。

注意文档的"正规则"概念：`pattern`/`kind` 匹配特定 kind 的节点，是正规则；`regex` 可以匹配任何满足正则的节点，不是。每条 rule 至少要有一个正规则。

### 4.2 Relational Rules（关系规则）——真正的杀手锏

- `inside`：目标位于某节点**之内**；
- `has`：目标**拥有**某后代节点；
- `follows` / `precedes`：目标在某节点之后/之前（兄弟序）。

```yaml
# 例：类字段名为 foo 的定义
has:
  kind: property_identifier
  field: name            # 约束具体字段
```

**`stopBy` 控制搜索半径**（仅关系规则有）：

- `neighbor`（默认）：紧邻节点不匹配即停止——快但浅；
- `end`：搜到底（inside 到根、has 到叶、follows 到首个兄弟）；
- 传 rule 对象：命中该规则处停（且含包容性语义）。

### 4.3 Composite Rules + 复用

`all` / `any` / `not` / `matches` 组合；`utils` 顶层定义具名规则供 `matches` 引用。语义要点："all/any refers to rules, not nodes"——组合的是规则，一次仍只匹配**单个**节点。

### 4.4 为什么我们不移植这套

对人的 lint 工作流，规则体系是核心价值；但对 **LLM 消费者**（我们的检索 MCP 服务于模型）：

- 模型天然擅长**多步组合**——`find_symbol` + `read_file` + `search_pattern` + `find_references` 串起来就是手写的 inside/has；
- 规则 DSL 意味着模型要先学一门语言再调试它，出错率高于直接写 pattern；
- 我们是只读工具集，codemod/rewrite 面整个不存在。

所以移植边界划在 pattern 语义这一层（见 §7 对照表）。

## 5. CLI 与生态

| 命令 | 用途 |
|---|---|
| `sg run` / `ast-grep -p '<pattern>'` | 结构化搜索（对标 grep） |
| `sg scan` | 按 YAML 规则集 lint（对标 eslint），规则可发布成包 |
| `sg scan --update-all` | `--rewrite` 自动改码（对标 codemod） |
| `sg test` | 规则作者的快照测试框架 |
| `sg new` | 交互式规则脚手架 |
| `sg outline` | 提取代码大纲（符号概览） |

性能：Rust + Tree-sitter + 并行，官方口径"万级文件秒级"，简单查询可快过 ag。另有 Language Server、Playground（网页版调试 pattern/rule）、Node/Python/WASM API。

## 6. 与 AI 结合（官方 advanced/prompting 页）

官方明确把 LLM 当一等用户，四种姿势：

1. **Claude Code Skill**：官方 skill 包教模型写 ast-grep 规则——典型查询："找没做错误处理的 async 函数""用了特定 hook 的组件"；
2. **AGENTS.md 提示词**：一行约定"结构相关的搜索默认用 `ast-grep --lang X -p '...'`"（依赖模型本身记得语法）；
3. **llms-full.txt 注入**：整本文档打进上下文，显著降低规则幻觉；
4. **ast-grep-mcp（实验性）**：MCP 服务器让模型试错迭代开发规则——导出 AST、测 pattern、逐步修。

他们总结的**规则开发六步**（拆解查询 → 识别子规则 → 组合 → 失败时删减调试 → 导出 AST → 试例验证）本质是"把人类调规则的过程交给 agent 循环"。

我们走的是姿势 4 的变体且更彻底：不包装外部 CLI，直接把结构化搜索做成基座自有的 MCP 工具（零外部依赖、Go 单二进制），LLM 用 `repo_search_pattern` 一步到位。

## 7. 与本基座实现的对照

`repotools/pattern.go`（`search_pattern` 工具）对 ast-grep 的取舍：

| 能力 | ast-grep | 本基座 | 说明 |
|---|---|---|---|
| 解析器 | Tree-sitter（20+ 语言） | `go/parser`（Go-only） | 基座场景是 Go 服务检索 |
| `$NAME` 单节点 | ✓ | ✓ | 同名要求渲染文本一致 |
| `$$$NAME` 列表 | ✓ | ✓ | 实参/形参/语句块，回溯展开 |
| `$_` 匿名 | ✓ | ✓ | 不捕获、无一致性约束 |
| 多语句 pattern | ✓ | ✓ | 块内连续语句子序列，最短前缀优先 |
| 对象式 pattern（context+selector） | ✓ | ✗ | Go 无 JS 式歧义，用不了 |
| `$$VAR` unnamed 节点 | ✓ | ✗ | Tree-sitter 概念，go/ast 不存在 |
| kind/regex/nthChild/range | ✓ | 部分（`find_symbol`≈kind，`search_code`≈regex） | 不同工具承担 |
| inside/has/follows + stopBy | ✓ | ✗ | LLM 多步组合替代（§4.4） |
| rewrite/codemod | ✓ | ✗ | 只读工具集，无写语义 |
| YAML 规则/utils 复用 | ✓ | ✗ | 同上 |

**Go 移植的三个实现要点**（踩过坑，值得记）：

1. 位置字段必须跳过：`token.Pos` 是 int 类型，反射比较时若不按类型跳过，pattern 和目标的源码位置永远不同；
2. 接口切片要解包：`[]ast.Expr` 的元素 `reflect.Value.Kind()` 是 `Interface` 不是 `Ptr`，不解包会回退到 `DeepEqual`（连位置一起比，恒 false）；
3. 节点互递归要有方向：`matchNode`（节点层，管元变量分发）必须直接下钻到 struct 值，`matchValue` 的 Ptr 分发只服务 struct 字段里的子节点——否则同一对节点 `matchNode ↔ matchValue` 无限互递归爆栈。

## 8. 后续可选方向

- **匹配语义升级**：目前同名一致性比较用 `printer` 渲染文本；若需忽略字面量等价（`0x10` vs `16`）可换成规范化 AST 哈希；
- **`$$$` 在返回值/字段列表**：`matchSeq` 已是通用序列匹配，扩展位置类型成本很低；
- **若要跨语言**：Tree-sitter 的 Go 绑定（`github.com/smacker/go-tree-sitter`）可平移整套匹配器设计——但目前基座检索对象就是 Go 服务，不急。

## 参考

- 官方文档：<https://ast-grep.github.io/>（guide/pattern-syntax、reference/rule、advanced/prompting 最值得读）
- 在线 Playground：<https://ast-grep.github.io/playground/>（调 pattern 神器）
- 书：《Mastering ast-grep》（Leanpub，作者自著）
- 本基座对应实现：`repotools/pattern.go`、`repotools/pattern_test.go`、`cmd/repo-mcp/main.go`

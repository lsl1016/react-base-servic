// pattern.go 提供 ast-grep 风格的结构化搜索（Go-only）：
// pattern 是一段 Go 源码（表达式或语句序列），元变量 $NAME 匹配任意单节点、
// $$$NAME 匹配列表上下文（实参/形参/语句块）中的零或多节点；同名元变量要求
// 源码文本一致（等价正则捕获组回引），$_ 前缀的元变量不捕获、多处可不同。
// 相比 search_code 的文本匹配：字符串/注释中的字面文本不会误命中，跨行与空白差异不敏感。
package repotools

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

var (
	multiVarRe = regexp.MustCompile(`\$\$\$([A-Z_][A-Z_0-9]*)`)
	oneVarRe   = regexp.MustCompile(`\$([A-Z_][A-Z_0-9]*)`)
)

// compiledPattern 是占位替换并解析后的 pattern；one/multi 记录占位 ident → 元变量名。
type compiledPattern struct {
	root  ast.Node // *ast.BlockStmt（语句序列，用 List）或 ast.Expr
	one   map[string]string
	multi map[string]string
}

// patternMatcher 绑定单次匹配的目标文件上下文（fset/源码用于渲染绑定文本）。
type patternMatcher struct {
	c    *compiledPattern
	fset *token.FileSet
	src  []byte
}

// SearchPattern 结构化搜索：在 rel 子树的 .go 文件里匹配 pattern。
func (r *Repo) SearchPattern(pattern, rel string, maxResults int) ([]map[string]any, error) {
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	if maxResults <= 0 || maxResults > 200 {
		maxResults = 50
	}
	compiled, err := compilePattern(pattern)
	if err != nil {
		return nil, err
	}
	results := []map[string]any{}
	err = r.walkParsedGoFiles(func(path string, fset *token.FileSet, file *ast.File, src []byte) bool {
		m := &patternMatcher{c: compiled, fset: fset, src: src}
		collect := func(start, end token.Pos, binds map[string]string) {
			if len(results) >= maxResults {
				return
			}
			off := fset.File(start).Offset(start)
			offEnd := fset.File(end).Offset(end)
			if off < 0 || offEnd > len(src) || off > offEnd {
				return
			}
			hm := map[string]any{
				"path":       filepath.ToSlash(mustRel(r.root, path)),
				"start_line": fset.Position(start).Line,
				"end_line":   fset.Position(end).Line,
				"match":      strings.TrimSpace(string(src[off:offEnd])),
			}
			if len(binds) > 0 {
				hm["metavars"] = binds
			}
			results = append(results, hm)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if len(results) >= maxResults {
				return false
			}
			switch root := compiled.root.(type) {
			case *ast.BlockStmt: // 语句序列 pattern：匹配块内每个起点的连续语句子序列（最短优先）
				if block, ok := n.(*ast.BlockStmt); ok {
					ps := toNodes(root.List)
					for i := 0; i < len(block.List); i++ {
						for end := i + 1; end <= len(block.List); end++ {
							binds := map[string]string{}
							if m.matchSeq(ps, toNodes(block.List[i:end]), binds) {
								collect(block.List[i].Pos(), block.List[end-1].End(), binds)
								break
							}
						}
					}
				}
			default: // 表达式 pattern：对每个节点尝试
				binds := map[string]string{}
				if n != nil && m.matchNode(compiled.root, n, binds) {
					collect(n.Pos(), n.End(), binds)
				}
			}
			return true
		})
		return len(results) < maxResults
	})
	return results, err
}

// compilePattern 替换元变量占位并解析为表达式或语句序列。
func compilePattern(src string) (*compiledPattern, error) {
	c := &compiledPattern{one: map[string]string{}, multi: map[string]string{}}
	translated := src
	translated = multiVarRe.ReplaceAllStringFunc(translated, func(s string) string {
		name := s[3:]
		id := fmt.Sprintf("__sgMulti%d__", len(c.multi))
		c.multi[id] = name
		return id
	})
	translated = oneVarRe.ReplaceAllStringFunc(translated, func(s string) string {
		name := s[1:]
		id := fmt.Sprintf("__sgOne%d__", len(c.one))
		c.one[id] = name
		return id
	})
	if expr, err := parser.ParseExpr(translated); err == nil {
		c.root = expr
		return c, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "pattern.go", "package p\nfunc _() {\n"+translated+"\n}\n", parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("pattern 不是可解析的 Go 表达式或语句: %v", err)
	}
	if len(file.Decls) == 0 {
		return nil, fmt.Errorf("pattern 解析为空")
	}
	fd, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || fd.Body == nil {
		return nil, fmt.Errorf("pattern 须为表达式或语句序列")
	}
	c.root = fd.Body
	return c, nil
}

// matchNode 匹配单个节点；pattern 侧占位 ident 走元变量分支。
func (m *patternMatcher) matchNode(p, t ast.Node, binds map[string]string) bool {
	if id, ok := p.(*ast.Ident); ok {
		if name, isOne := m.c.one[id.Name]; isOne {
			text := m.render(t)
			if prev, seen := binds[name]; seen {
				return prev == text
			}
			if strings.HasPrefix(name, "_") {
				return true // $_ 匿名：不捕获，无一致性约束
			}
			binds[name] = text
			return true
		}
		if _, isMulti := m.c.multi[id.Name]; isMulti {
			return false // $$$ 只在列表位置有效
		}
	}
	pv, tv := reflect.ValueOf(p), reflect.ValueOf(t)
	if pv.Type() != tv.Type() {
		return false
	}
	// 直接下钻到 struct 值；Ptr 分支的 asNode 分发只服务 struct 字段里的子节点，
	// 否则会在同一对节点上 matchNode ↔ matchValue 无限互递归。
	if pv.Kind() == reflect.Ptr {
		if pv.IsNil() || tv.IsNil() {
			return pv.IsNil() && tv.IsNil()
		}
		return m.matchValue(pv.Elem(), tv.Elem(), binds)
	}
	return m.matchValue(pv, tv, binds)
}

// matchValue 用反射对同类型 AST 值做结构比较；位置/注释等非结构字段跳过。
func (m *patternMatcher) matchValue(p, t reflect.Value, binds map[string]string) bool {
	// token.Pos 是 int、*ast.Object 等是指针——跳过判断必须在入口做，不能只在 Struct 分支
	if skipFieldType(p.Type()) {
		return true
	}
	switch p.Kind() {
	case reflect.Ptr, reflect.Interface:
		if p.IsNil() || t.IsNil() {
			return p.IsNil() && t.IsNil()
		}
		if pn, ok := asNode(p); ok {
			if tn, ok2 := asNode(t); ok2 {
				return m.matchNode(pn, tn, binds)
			}
		}
		return m.matchValue(p.Elem(), t.Elem(), binds)
	case reflect.Struct:
		for i := 0; i < p.NumField(); i++ {
			if !m.matchValue(p.Field(i), t.Field(i), binds) {
				return false
			}
		}
		return true
	case reflect.Slice:
		if p.Len() == 0 && t.Len() == 0 {
			return true
		}
		if ps, ok := toNodeSlice(p); ok {
			ts, _ := toNodeSlice(t)
			return m.matchSeq(ps, ts, binds)
		}
		return p.Len() == t.Len() && reflect.DeepEqual(p.Interface(), t.Interface())
	default: // string / int / bool / token.Token 等
		return p.Interface() == t.Interface()
	}
}

// matchSeq 序列匹配：支持 $$$ 占位展开零或多元素（回溯保证正确性）。
func (m *patternMatcher) matchSeq(ps, ts []ast.Node, binds map[string]string) bool {
	if len(ps) == 0 {
		return len(ts) == 0
	}
	if id, ok := ps[0].(*ast.Ident); ok {
		if name, isMulti := m.c.multi[id.Name]; isMulti {
			for k := 0; k <= len(ts); k++ {
				b2 := cloneBinds(binds)
				if !strings.HasPrefix(name, "_") {
					text := m.renderSeq(ts[:k])
					if prev, seen := b2[name]; seen {
						if prev != text {
							continue
						}
					} else {
						b2[name] = text
					}
				}
				if m.matchSeq(ps[1:], ts[k:], b2) {
					copyBinds(b2, binds)
					return true
				}
			}
			return false
		}
	}
	if len(ts) == 0 {
		return false
	}
	return m.matchNode(ps[0], ts[0], binds) && m.matchSeq(ps[1:], ts[1:], binds)
}

// walkParsedGoFiles 遍历 rel 子树解析 .go 文件；回调返回 false 时提前终止。
func (r *Repo) walkParsedGoFiles(fn func(path string, fset *token.FileSet, file *ast.File, src []byte) bool) error {
	return filepath.WalkDir(r.root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if path != r.root && ignored(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxGoSourceBytes {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			return nil // 语法不完整/生成文件：跳过
		}
		if !fn(path, fset, file, src) {
			return fs.SkipAll
		}
		return nil
	})
}

// render 把节点渲染为规范化单行文本（同名元变量一致性比较用）。
func (m *patternMatcher) render(n ast.Node) string {
	var b strings.Builder
	if err := printer.Fprint(&b, m.fset, n); err != nil {
		return ""
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func (m *patternMatcher) renderSeq(ns []ast.Node) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = m.render(n)
	}
	return strings.Join(parts, " ")
}

func asNode(v reflect.Value) (ast.Node, bool) {
	// []ast.Expr 等接口切片的元素是 Interface Kind，先解包到具体指针
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil, false
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		n, ok := v.Interface().(ast.Node)
		return n, ok
	}
	return nil, false
}

// toNodeSlice 把 []ast.Stmt / []ast.Expr / []ast.Field 等节点切片转成 []ast.Node。
func toNodeSlice(v reflect.Value) ([]ast.Node, bool) {
	if v.Kind() != reflect.Slice {
		return nil, false
	}
	out := make([]ast.Node, v.Len())
	for i := 0; i < v.Len(); i++ {
		n, ok := asNode(v.Index(i))
		if !ok {
			return nil, false
		}
		out[i] = n
	}
	return out, true
}

func toNodes[T ast.Node](ns []T) []ast.Node {
	out := make([]ast.Node, len(ns))
	for i, n := range ns {
		out[i] = n
	}
	return out
}

// skipFieldType 跳过位置、注释、符号表等非结构字段。
func skipFieldType(t reflect.Type) bool {
	switch t.String() {
	case "token.Pos", "*ast.Object", "*ast.Scope", "*ast.Comment", "*ast.CommentGroup", "[]*ast.Comment":
		return true
	}
	return false
}

func cloneBinds(b map[string]string) map[string]string {
	out := make(map[string]string, len(b))
	for k, v := range b {
		out[k] = v
	}
	return out
}

func copyBinds(src, dst map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}

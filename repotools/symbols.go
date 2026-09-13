// symbols.go 提供基于 go/ast 的声明感知检索（参考 repo-pilot 插件的语义强度契约）：
// find_symbol / get_file_symbols 返回真实的 Go 声明（含签名与接收者），不是文本匹配；
// find_references 是整词文本检索，返回候选用法而非已证实的调用者。
package repotools

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// maxGoSourceBytes 超过此大小的 .go 文件跳过 AST 解析（与读取上限一致）。
const maxGoSourceBytes = 2 << 20

// Symbol 描述一个 Go 顶层声明。
type Symbol struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"` // func / method / type / var / const
	Name      string `json:"name"`
	Receiver  string `json:"receiver,omitempty"` // method 的接收者类型
	Signature string `json:"signature,omitempty"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line,omitempty"`
}

// FileSymbols 解析单个 .go 文件，返回其顶层声明（含接收者与签名）。
func (r *Repo) FileSymbols(rel string) ([]Symbol, error) {
	path, err := r.safeExisting(rel)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(path); err != nil {
		return nil, err
	} else if info.IsDir() || info.Size() > maxGoSourceBytes {
		return nil, fmt.Errorf("not a parsable go file:%s", rel)
	}
	syms, err := parseGoFile(r.root, path)
	if err != nil {
		return nil, err
	}
	return syms, nil
}

// FindSymbol 定位与 name 匹配的 Go 声明。先做声明名精确匹配；无命中时回退到
// 声明名子串匹配；仍无命中时回退到文本检索（与旧词法行为兼容）。
func (r *Repo) FindSymbol(name string, maxResults int) ([]map[string]any, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if maxResults <= 0 || maxResults > 200 {
		maxResults = 50
	}
	var exact, partial []Symbol
	err := r.walkGoFiles(func(syms []Symbol) bool {
		for _, s := range syms {
			target := s.Name
			if s.Receiver != "" {
				target = s.Receiver + "." + s.Name
			}
			switch {
			case target == name || s.Name == name:
				exact = append(exact, s)
			case strings.Contains(target, name):
				partial = append(partial, s)
			}
			if len(exact) >= maxResults {
				return false
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	found := exact
	if len(found) < maxResults {
		found = append(found, partial[:min(len(partial), maxResults-len(found))]...)
	}
	if len(found) == 0 {
		return r.SearchCode(name, ".", maxResults)
	}
	out := make([]map[string]any, len(found))
	for i, s := range found {
		out[i] = symbolMap(s)
	}
	return out, nil
}

// FindReferences 用整词匹配检索标识符的候选用法（非语义引用）。
func (r *Repo) FindReferences(name, rel string, maxResults int) ([]map[string]any, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if !isIdentifier(name) {
		return nil, fmt.Errorf("name is not a go identifier")
	}
	if maxResults <= 0 || maxResults > 200 {
		maxResults = 50
	}
	re, err := regexp.Compile(`\b` + regexp.QuoteMeta(name) + `\b`)
	if err != nil {
		return nil, err
	}
	results := []map[string]any{}
	err = r.searchLines(rel, func(relPath string, line int, text string) bool {
		if re.MatchString(text) {
			results = append(results, map[string]any{
				"path": relPath, "line": line, "content": strings.TrimSpace(text),
			})
		}
		return len(results) < maxResults
	})
	return results, err
}

// symbolMap 把 Symbol 转为工具返回的 map（声明命中统一带 symbol=true 标记）。
func symbolMap(s Symbol) map[string]any {
	m := map[string]any{
		"path": s.Path, "kind": s.Kind, "name": s.Name,
		"start_line": s.StartLine, "symbol": true,
	}
	if s.Receiver != "" {
		m["receiver"] = s.Receiver
	}
	if s.Signature != "" {
		m["signature"] = s.Signature
	}
	if s.EndLine > 0 {
		m["end_line"] = s.EndLine
	}
	return m
}

// walkGoFiles 遍历仓库内全部 .go 文件做 AST 解析；回调返回 false 时提前终止。
func (r *Repo) walkGoFiles(fn func(syms []Symbol) bool) error {
	return r.walkParsedGoFiles(func(path string, fset *token.FileSet, file *ast.File, src []byte) bool {
		return fn(symbolsFromDecls(filepath.ToSlash(mustRel(r.root, path)), fset, file.Decls))
	})
}

// parseGoFile 解析单个 .go 文件并提取顶层声明摘要。
func parseGoFile(root, path string) ([]Symbol, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	return symbolsFromDecls(filepath.ToSlash(mustRel(root, path)), fset, file.Decls), nil
}

// symbolsFromDecls 从声明列表提取符号摘要。
func symbolsFromDecls(relPath string, fset *token.FileSet, decls []ast.Decl) []Symbol {
	syms := []Symbol{}
	for _, decl := range decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			s := Symbol{
				Path:      relPath,
				Kind:      "func",
				Name:      d.Name.Name,
				StartLine: fset.Position(d.Pos()).Line,
				EndLine:   fset.Position(d.End()).Line,
				Signature: "func " + formatNode(fset, d.Type),
			}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				s.Kind = "method"
				s.Receiver = receiverType(fset, d)
				s.Signature = "func (" + strings.TrimSpace(formatNode(fset, d.Recv.List[0].Type)) + ") " + d.Name.Name + " " + formatNode(fset, d.Type)
			}
			syms = append(syms, s)
		case *ast.GenDecl:
			kind := map[token.Token]string{token.TYPE: "type", token.VAR: "var", token.CONST: "const"}[d.Tok]
			if kind == "" {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					if vs, ok2 := spec.(*ast.ValueSpec); ok2 && len(vs.Names) > 0 {
						syms = append(syms, Symbol{Path: relPath, Kind: kind, Name: vs.Names[0].Name, StartLine: fset.Position(spec.Pos()).Line})
					}
					continue
				}
				syms = append(syms, Symbol{
					Path: relPath, Kind: kind, Name: ts.Name.Name,
					Signature: "type " + ts.Name.Name + " " + formatNode(fset, ts.Type),
					StartLine: fset.Position(spec.Pos()).Line,
					EndLine:   fset.Position(spec.End()).Line,
				})
			}
		}
	}
	return syms
}

// GoRepoMapLine 生成一个 .go 文件的声明摘要行（Aider 风格 repo map）。
func GoRepoMapLine(root, path string, maxDecls int) (string, bool) {
	syms, err := parseGoFile(root, path)
	if err != nil || len(syms) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(syms))
	for _, s := range syms {
		if len(parts) >= maxDecls {
			parts = append(parts, "...")
			break
		}
		switch s.Kind {
		case "method":
			parts = append(parts, fmt.Sprintf("%s.%s", s.Receiver, s.Name))
		case "func", "type":
			parts = append(parts, s.Name)
		default: // var/const 噪声大，repo map 不带
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return filepath.ToSlash(mustRel(root, path)) + ": " + strings.Join(parts, ", "), true
}

// ListRepositories 列出仓库根下的顶层目录（镜像目录形态：每个子目录一个项目）。
func (r *Repo) ListRepositories() []string {
	entries, err := os.ReadDir(r.root)
	if err != nil {
		return nil
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() && !ignored(e.Name()) {
			out = append(out, e.Name()+"/")
		}
	}
	sort.Strings(out)
	return out
}

// formatNode 把 AST 节点压成单行签名片段。
func formatNode(fset *token.FileSet, node any) string {
	var b strings.Builder
	if err := printer.Fprint(&b, fset, node); err != nil {
		return ""
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// receiverType 提取方法接收者的类型名（去指针/泛型参数）。
func receiverType(fset *token.FileSet, d *ast.FuncDecl) string {
	t := formatNode(fset, d.Recv.List[0].Type)
	t = strings.TrimPrefix(t, "*")
	if i := strings.IndexByte(t, '['); i > 0 {
		t = t[:i]
	}
	return t
}

func mustRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

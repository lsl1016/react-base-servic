// repo-mcp 是一个只读本地仓库的 MCP stdio 服务器。
//
// 启动方式：被 react-base-service 的 MCP 客户端（service/mcpclient）作为子进程拉起，
// 环境变量 REPO_ROOT 指定仓库根目录（默认 /workspace/repo）。无任何鉴权，仅限本机受信环境。
package main

import (
	"fmt"
	"os"

	"react-base-service/mcpserver"
	"react-base-service/repotools"
)

func main() {
	root := os.Getenv("REPO_ROOT")
	if root == "" {
		root = "/workspace/repo"
	}
	repo, err := repotools.New(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open repo failed:%v\n", err)
		os.Exit(1)
	}
	server := mcpserver.New("repo-readonly", tools(repo))
	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "repo mcp failed:%v\n", err)
		os.Exit(1)
	}
}

func tools(repo *repotools.Repo) []mcpserver.Tool {
	return []mcpserver.Tool{
		{Name: "list_repositories", Description: "List top-level project directories (mirrors) under the configured read-only search root.", InputSchema: object(map[string]any{}), Handler: func(a map[string]any) (any, error) {
			return repo.ListRepositories(), nil
		}},
		{Name: "list_files", Description: "List files/directories under the configured read-only repository root.", InputSchema: object(map[string]any{"path": stringSchema(), "max_entries": integerSchema()}), Handler: func(a map[string]any) (any, error) {
			return repo.ListFiles(text(a, "path"), integer(a, "max_entries", 200))
		}},
		{Name: "read_file", Description: "Read a bounded line range from one repository file.", InputSchema: object(map[string]any{"path": stringSchema(), "start_line": integerSchema(), "end_line": integerSchema()}, "path"), Handler: func(a map[string]any) (any, error) {
			return repo.ReadFile(text(a, "path"), integer(a, "start_line", 1), integer(a, "end_line", 200))
		}},
		{Name: "search_code", Description: "Search source text for case-insensitive substring matches; returns bounded candidates.", InputSchema: object(map[string]any{"query": stringSchema(), "path": stringSchema(), "max_results": integerSchema()}, "query"), Handler: func(a map[string]any) (any, error) {
			return repo.SearchCode(text(a, "query"), text(a, "path"), integer(a, "max_results", 50))
		}},
		{Name: "search_pattern", Description: "ast-grep style structural search over Go syntax trees: pattern is a Go expression or statement sequence; $NAME matches any single node, $$$NAME matches zero-or-more list items (args/params/statements); same-named metavars must match identical text; $_-prefixed metavars are anonymous. Unlike text search, literals in strings/comments never match.", InputSchema: object(map[string]any{"pattern": stringSchema(), "path": stringSchema(), "max_results": integerSchema()}, "pattern"), Handler: func(a map[string]any) (any, error) {
			return repo.SearchPattern(text(a, "pattern"), text(a, "path"), integer(a, "max_results", 50))
		}},
		{Name: "find_symbol", Description: "Locate Go declarations (func/method/type) by name using go/ast, with signatures and line ranges; falls back to lexical search only when no declaration matches.", InputSchema: object(map[string]any{"name": stringSchema(), "max_results": integerSchema()}, "name"), Handler: func(a map[string]any) (any, error) {
			return repo.FindSymbol(text(a, "name"), integer(a, "max_results", 50))
		}},
		{Name: "get_file_symbols", Description: "List top-level Go declarations (kinds, receivers, signatures, line ranges) of one .go file.", InputSchema: object(map[string]any{"path": stringSchema()}, "path"), Handler: func(a map[string]any) (any, error) {
			return repo.FileSymbols(text(a, "path"))
		}},
		{Name: "find_references", Description: "Whole-word search for a Go identifier's candidate usages. Results are candidates, not verified semantic references.", InputSchema: object(map[string]any{"name": stringSchema(), "path": stringSchema(), "max_results": integerSchema()}, "name"), Handler: func(a map[string]any) (any, error) {
			return repo.FindReferences(text(a, "name"), text(a, "path"), integer(a, "max_results", 50))
		}},
		{Name: "get_repo_map", Description: "Return a bounded repository map; .go files carry a top-level declaration summary (Aider-style) before deep reading.", InputSchema: object(map[string]any{"max_files": integerSchema()}), Handler: func(a map[string]any) (any, error) { return repo.RepoMap(integer(a, "max_files", 300)) }},
	}
}

func object(props map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}
func stringSchema() map[string]any { return map[string]any{"type": "string"} }
func integerSchema() map[string]any {
	return map[string]any{"type": "integer", "minimum": 1, "maximum": 2000}
}
func text(a map[string]any, k string) string {
	v, _ := a[k].(string)
	return v
}
func integer(a map[string]any, k string, d int) int {
	if v, ok := a[k].(float64); ok {
		return int(v)
	}
	return d
}

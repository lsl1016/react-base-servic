package repotools

import (
	"os"
	"path/filepath"
	"testing"
)

// 写入临时仓库结构，验证声明感知检索行为。
func setupSymbolRepo(t *testing.T) *Repo {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("svc/main.go", `package svc

type Client struct{ base string }

func (c *Client) DoWork(id int) error { return nil }

func NewClient() *Client { return &Client{} }

const maxRetry = 3
`)
	write("svc/util.go", `package svc

// CandidateListener 仅用于验证注释里的 Client 不算声明。
type CandidateListener interface {
	OnDone()
}
`)
	write("notes.txt", "Client mentioned in plain text only")
	write("sub/deep.go", `package sub

func Client() {} // 与类型同名的函数
`)
	repo, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestFindSymbolDeclarations(t *testing.T) {
	repo := setupSymbolRepo(t)

	got, err := repo.FindSymbol("DoWork", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("DoWork: want 1 declaration, got %v", got)
	}
	m := got[0]
	if m["kind"] != "method" || m["receiver"] != "Client" || m["path"] != "svc/main.go" {
		t.Fatalf("unexpected symbol: %v", m)
	}
	if sig, _ := m["signature"].(string); sig == "" {
		t.Fatalf("method should carry signature: %v", m)
	}

	// 精确命中时不应把注释/纯文本里的 Client 当声明返回
	got, err = repo.FindSymbol("Client", 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range got {
		if m["path"] == "notes.txt" {
			t.Fatalf("plain text leaked into declaration results: %v", m)
		}
	}
	if len(got) == 0 {
		t.Fatal("Client should match type + same-name func")
	}
}

func TestFindReferencesWholeWord(t *testing.T) {
	repo := setupSymbolRepo(t)

	got, err := repo.FindReferences("maxRetry", ".", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["path"] != "svc/main.go" {
		t.Fatalf("maxRetry refs: %v", got)
	}

	// 整词匹配：Candidate 不应命中 CandidateListener
	got, err = repo.FindReferences("Candidate", ".", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("whole-word violated, got %v", got)
	}
}

func TestFileSymbolsAndRepoMap(t *testing.T) {
	repo := setupSymbolRepo(t)

	syms, err := repo.FileSymbols("svc/main.go")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, s := range syms {
		kinds[s.Kind]++
	}
	if kinds["type"] != 1 || kinds["method"] != 1 || kinds["func"] != 1 || kinds["const"] != 1 {
		t.Fatalf("unexpected symbols: %+v", syms)
	}

	lines, err := repo.RepoMap(100)
	if err != nil {
		t.Fatal(err)
	}
	var goWithDecls, plain int
	for _, l := range lines {
		if l == "notes.txt" {
			plain++
		}
		if len(l) > len("svc/main.go") && l[:len("svc/main.go")] == "svc/main.go" && l != "svc/main.go" {
			goWithDecls++
		}
	}
	if goWithDecls != 1 || plain != 1 {
		t.Fatalf("repo map should annotate go files and keep plain files: %v", lines)
	}
}

func TestListRepositories(t *testing.T) {
	repo := setupSymbolRepo(t)
	repos := repo.ListRepositories()
	if len(repos) != 2 || repos[0] != "sub/" || repos[1] != "svc/" {
		t.Fatalf("want [sub/ svc/], got %v", repos)
	}
}

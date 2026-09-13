package repotools

import (
	"os"
	"path/filepath"
	"testing"
)

func setupPatternRepo(t *testing.T) *Repo {
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
	write("a/call.go", `package a

import "context"

func callSite() {
	logger.Error(ctx, "msg", 1)
	doThing(context.Background())
	s := logger.Error(ctx, "other", 2, 3)
	_ = s
}

func doThing(ctx context.Context) {}
`)
	write("a/nest.go", `package a

func nested() {
	// logger.Error(ctx, "in comment") 不应命中
	s := "logger.Error(ctx, \"in string\")"
	_ = s
}
`)
	write("a/stmt.go", `package a

func statements() error {
	err := step1()
	if err != nil {
		return err
	}
	return nil
}

func step1() error { return nil }
`)
	write("a/eq.go", `package a

func eq(a, b int) bool {
	return a == a && a == b
}
`)
	repo, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestSearchPatternExpression(t *testing.T) {
	repo := setupPatternRepo(t)

	// 表达式 pattern：命中两处调用，注释/字符串里的字面文本不命中
	got, err := repo.SearchPattern(`logger.Error($CTX, $MSG, $$$REST)`, ".", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 call sites, got %d: %v", len(got), got)
	}
	first := got[0]
	if first["path"] != "a/call.go" {
		t.Fatalf("unexpected path: %v", first)
	}
	vars := first["metavars"].(map[string]string)
	if vars["MSG"] != `"msg"` {
		t.Fatalf("metavar MSG = %q", vars["MSG"])
	}
	if vars["REST"] == "" { // 第一处无多余实参，REST 应为空绑定
		t.Fatalf("REST should be bound (possibly empty): %v", vars)
	}
}

func TestSearchPatternSameNameConstraint(t *testing.T) {
	repo := setupPatternRepo(t)

	// 同名 $A == $A 只匹配 a == a；$_X == $_X 匹配两处
	strict, err := repo.SearchPattern(`$A == $A`, ".", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(strict) != 1 {
		t.Fatalf("same-name: want 1, got %v", strict)
	}
	anon, err := repo.SearchPattern(`$_X == $_X`, ".", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(anon) != 2 {
		t.Fatalf("anonymous: want 2, got %v", anon)
	}
}

func TestSearchPatternStatements(t *testing.T) {
	repo := setupPatternRepo(t)

	got, err := repo.SearchPattern(`if err != nil {
	return $ERR
}`, ".", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("statement pattern: want 1, got %v", got)
	}
	if got[0]["metavars"].(map[string]string)["ERR"] != "err" {
		t.Fatalf("ERR binding: %v", got[0])
	}
}

func TestSearchPatternCompileErrors(t *testing.T) {
	repo := setupPatternRepo(t)
	if _, err := repo.SearchPattern(`func func`, ".", 10); err == nil {
		t.Fatal("invalid pattern should fail to compile")
	}
}

package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"react-base-service/conf"
)

// newTestRepo 在临时目录创建一个含两个 commit 的本地 git 仓库，返回其路径。
func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	run("-C", dir, "init", "-b", "main")
	run("-C", dir, "config", "user.email", "test@example.com")
	run("-C", dir, "config", "user.name", "test")
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello alpha"), 0o644)
	run("-C", dir, "add", ".")
	run("-C", dir, "commit", "-m", "first")
	_ = os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\nfunc Beta() {}\n"), 0o644)
	run("-C", dir, "add", ".")
	run("-C", dir, "commit", "-m", "second")
	return dir
}

// newTestManager 构造不触发单例的测试 Manager（独立临时目录）。
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return &Manager{
		root:    filepath.Join(t.TempDir(), "ws"),
		mirrors: filepath.Join(t.TempDir(), "mirror"),
		active:  map[string][]*Allocation{},
	}
}

func TestMirrorAndWorktreeLifecycle(t *testing.T) {
	repo := newTestRepo(t)
	m := newTestManager(t)

	mirror, err := m.ensureMirror(repo)
	if err != nil {
		t.Fatalf("mirror 克隆失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mirror, "HEAD")); err != nil {
		t.Fatalf("mirror 缺少 HEAD: %v", err)
	}

	// 再次 ensureMirror 走 fetch 刷新路径。
	if _, err := m.ensureMirror(repo); err != nil {
		t.Fatalf("mirror 刷新失败: %v", err)
	}

	// ref 解析：空（HEAD）与分支名。
	commit, err := m.resolveCommit(mirror, "")
	if err != nil || len(commit) != 40 {
		t.Fatalf("HEAD 解析失败: %q, %v", commit, err)
	}
	branchCommit, err := m.resolveCommit(mirror, "main")
	if err != nil || branchCommit != commit {
		t.Fatalf("main 解析失败: %q vs %q, %v", branchCommit, commit, err)
	}
	if _, err := m.resolveCommit(mirror, "main; rm -rf /"); err == nil {
		t.Fatal("含元字符的 ref 必须被拒绝")
	}

	// worktree 分配与释放。
	worktree := filepath.Join(m.root, "run_x", "svc")
	if err := m.git(mirror, "worktree", "add", "--detach", worktree, commit); err != nil {
		t.Fatalf("worktree 创建失败: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(worktree, "a.txt")); err != nil || string(data) != "hello alpha" {
		t.Fatalf("worktree 内容不符: %q, %v", data, err)
	}
	if err := m.git(mirror, "worktree", "remove", "--force", worktree); err != nil {
		t.Fatalf("worktree 移除失败: %v", err)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("worktree 目录应已移除: %v", err)
	}
}

func TestGitSubcommandWhitelist(t *testing.T) {
	m := newTestManager(t)
	if err := m.git("", "push", "origin", "main"); err == nil {
		t.Fatal("白名单外子命令必须被拒绝")
	}
	if _, err := m.gitOutput("", "log"); err == nil {
		t.Fatal("白名单外子命令（log）必须被拒绝")
	}
}

func TestResolveWhitelist(t *testing.T) {
	original := conf.CustomConf.LLM.React.Workspace
	defer func() { conf.CustomConf.LLM.React.Workspace = original }()
	conf.CustomConf.LLM.React.Workspace = conf.ReactWorkspaceConfig{
		Resolvers: []conf.ReactWorkspaceResolverConf{
			{Service: "svc-a", RepoURL: "https://git.example.com/svc-a.git", DefaultRef: "main", Refs: map[string]string{"prod": "release-1.0"}},
		},
	}

	if url, ref, err := Resolve("svc-a", "prod"); err != nil || url != "https://git.example.com/svc-a.git" || ref != "release-1.0" {
		t.Fatalf("env 命中解析失败: %s %s %v", url, ref, err)
	}
	if _, ref, err := Resolve("svc-a", ""); err != nil || ref != "main" {
		t.Fatalf("默认 ref 解析失败: %s %v", ref, err)
	}
	if _, ref, err := Resolve("svc-a", "unknown-env"); err != nil || ref != "main" {
		t.Fatalf("未知 env 应回退默认 ref: %s %v", ref, err)
	}
	if _, _, err := Resolve("svc-not-exist", ""); err == nil {
		t.Fatal("白名单外 service 必须被拒绝")
	}
}

func TestMirrorPathSanitized(t *testing.T) {
	m := newTestManager(t)
	if got := m.mirrorPath("https://git.example.com/weird/../name.git"); strings.Contains(got, "..") {
		t.Fatalf("mirror 路径未净化: %s", got)
	}
	if got := m.mirrorPath("https://git.example.com/ok.git"); !strings.HasSuffix(got, "ok.git") {
		t.Fatalf("mirror 路径不符: %s", got)
	}
}

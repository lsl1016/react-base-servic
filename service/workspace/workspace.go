// Package workspace 实现服务端代码工作区（P2-1，方案 §4.2.1）：
//
//	A. RuntimeSourceResolver：service+env → repo_url+ref。P2 首版为静态配置白名单
//	   （llm.react.workspace.resolvers），线上镜像 digest → commit 的真实解析后续接公司基建；
//	B. RepoMirrorCache：bare mirror 单副本缓存（git clone --mirror / fetch --prune），
//	   全部 worktree 共享对象库，避免每次完整 clone；
//	C. WorkspaceAllocator：每 run 一个 git worktree（秒级、按 commit 精确锁定），
//	   run 终态时统一 release（worktree remove + 目录清理 + 动态 MCP 卸载）；
//	D. 挂载：为该 run 动态实例化只读 repo MCP（复用 mcpclient stdio 适配器，
//	   REPO_ROOT=worktree，工具同步为 caller 名下副本，工具名前缀 = ws_<service>_）。
//
// 安全面：service/env 仅允许字母数字下划线中划线（防路径穿越）；ref 来自配置白名单并经
// refPattern 字符校验；git 子命令有白名单；全部动态参数进入子进程前均做过字符正则校验
// （禁止空白与 shell 元字符），且不经 shell 执行；resolver 静态白名单不接收任意仓库地址；
// worktree 只读语义由 repo-mcp 工具集保证（无写工具）。
package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"react-base-service/components"
	"react-base-service/conf"
	"react-base-service/golib/zlog"
	"react-base-service/service/mcpclient"

	"github.com/gin-gonic/gin"
)

var (
	servicePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	// refPattern 是允许进入 git argv 的 ref 字符（分支/tag/commit 常见字符），拒绝空白与元字符。
	refPattern = regexp.MustCompile(`^[a-zA-Z0-9._/\-]+$`)
	// commitPattern 是完整 commit 哈希格式（rev-parse 输出校验）。
	commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	// gitSubcommands 是允许执行的 git 子命令白名单。
	gitSubcommands = map[string]bool{
		"clone":     true,
		"fetch":     true,
		"worktree":  true,
		"rev-parse": true,
	}
)

// Allocation 是一次成功的工作区分配。
type Allocation struct {
	Service string `json:"service"`
	Env     string `json:"env"`
	RepoURL string `json:"repoUrl"`
	Ref     string `json:"ref"`
	Commit  string `json:"commit"`
	Path    string `json:"path"`
	// MCPServerName 是动态挂载的 repo MCP 名（工具名前缀 = <name>_）。
	MCPServerName string `json:"mcpServerName"`
	// CallerKey 是工具副本同步的归属 caller（release 时按此清理）。
	CallerKey string `json:"callerKey"`
}

// Manager 管理工作区分配与释放；进程内单例（Default()）。
type Manager struct {
	mu      sync.Mutex
	root    string
	mirrors string
	active  map[string][]*Allocation // runID → allocations
}

var (
	defaultManager *Manager
	defaultOnce    sync.Once
)

// Default 返回进程级单例（首次调用时清理上次进程遗留的孤儿 worktree 目录）。
func Default() *Manager {
	defaultOnce.Do(func() {
		cfg := conf.GetReactRuntimeConfig().Workspace
		root, rootErr := filepath.Abs(cfg.RootDir)
		mirrors, mirrorsErr := filepath.Abs(cfg.MirrorDir)
		if rootErr != nil || mirrorsErr != nil {
			zlog.Warnf(nil, "[Workspace] 目录绝对化失败(使用原值): rootErr=%v, mirrorsErr=%v", rootErr, mirrorsErr)
			root, mirrors = cfg.RootDir, cfg.MirrorDir
		}
		defaultManager = &Manager{
			root:    root,
			mirrors: mirrors,
			active:  map[string][]*Allocation{},
		}
		defaultManager.cleanupOrphans()
	})
	return defaultManager
}

// Load 解析服务代码并分配 worktree + 挂载 repo MCP。同一 run 内同一 service 幂等。
func (m *Manager) Load(ctx *gin.Context, runID, callerKey, service, env string) (*Allocation, error) {
	if !conf.GetReactRuntimeConfig().Workspace.WorkspaceEnabled() {
		return nil, components.ErrorParamInvalid.Sprintf("workspace 未启用（llm.react.workspace.enabled）")
	}
	service = strings.TrimSpace(service)
	env = strings.TrimSpace(env)
	if service == "" || !servicePattern.MatchString(service) {
		return nil, components.ErrorParamInvalid.Sprintf("service 仅允许字母数字下划线中划线: %q", service)
	}
	if env != "" && !servicePattern.MatchString(env) {
		return nil, components.ErrorParamInvalid.Sprintf("env 仅允许字母数字下划线中划线: %q", env)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, alloc := range m.active[runID] {
		if alloc.Service == service {
			return alloc, nil
		}
	}

	repoURL, ref, err := Resolve(service, env)
	if err != nil {
		return nil, err
	}

	mirrorPath, err := m.ensureMirror(repoURL)
	if err != nil {
		return nil, err
	}
	commit, err := m.resolveCommit(mirrorPath, ref)
	if err != nil {
		return nil, err
	}

	// worktree 必须用绝对路径：git 在 mirror 目录内执行，相对路径会相对 mirror 解析。
	worktree, err := filepath.Abs(filepath.Join(m.root, runID, service))
	if err != nil {
		return nil, err
	}
	if err := m.git(mirrorPath, "worktree", "add", "--detach", worktree, commit); err != nil {
		// 已存在（异常残留）时先移除再重试一次。
		_ = m.git(mirrorPath, "worktree", "remove", "--force", worktree)
		_ = m.git(mirrorPath, "worktree", "add", "--detach", worktree, commit)
	}

	// 动态挂载只读 repo MCP 并同步工具副本到该 caller（工具名 ws_<service>_<tool>）。
	serverName := "ws_" + service
	if _, err := mcpclient.EnsureServer(mcpclient.ServerConfig{
		Name: serverName,
		Kind: "repo",
		Env:  map[string]string{"REPO_ROOT": worktree},
	}); err != nil {
		_ = m.git(mirrorPath, "worktree", "remove", "--force", worktree)
		return nil, components.ErrorToolExecFailed.Sprintf("挂载 repo MCP 失败: %v", err)
	}
	if _, err := mcpclient.SyncServerRegistryScoped(callerKey, serverName, false); err != nil {
		zlog.Warnf(ctx, "[Workspace] 工具同步失败(尝试继续): server=%s, err=%v", serverName, err)
	}

	alloc := &Allocation{
		Service:       service,
		Env:           env,
		RepoURL:       repoURL,
		Ref:           ref,
		Commit:        commit,
		Path:          worktree,
		MCPServerName: serverName,
		CallerKey:     callerKey,
	}
	m.active[runID] = append(m.active[runID], alloc)
	zlog.Infof(ctx, "[Workspace] 工作区就绪: runId=%s, service=%s@%s(ref=%s), path=%s, tools=%s_*", runID, service, commit, ref, worktree, serverName)
	return alloc, nil
}

// ReleaseRun 释放一个 run 的全部工作区（幂等，run 终态统一调用）。
func (m *Manager) ReleaseRun(ctx *gin.Context, runID string) {
	m.mu.Lock()
	allocs := m.active[runID]
	delete(m.active, runID)
	m.mu.Unlock()

	for _, alloc := range allocs {
		m.release(ctx, alloc)
	}
}

func (m *Manager) release(ctx *gin.Context, alloc *Allocation) {
	// 先摘工具与 MCP 客户端，再回收 worktree，模型侧 get_tool 不再命中。
	if _, err := mcpclient.RemoveRegistryToolsForCaller(ctx, alloc.MCPServerName, alloc.CallerKey); err != nil {
		zlog.Warnf(ctx, "[Workspace] 清理工具副本失败(忽略): server=%s, err=%v", alloc.MCPServerName, err)
	}
	if err := mcpclient.RemoveServer(alloc.MCPServerName); err != nil {
		zlog.Warnf(ctx, "[Workspace] 停止 repo MCP 失败(忽略): server=%s, err=%v", alloc.MCPServerName, err)
	}
	mirrorPath := m.mirrorPath(alloc.RepoURL)
	if err := m.git(mirrorPath, "worktree", "remove", "--force", alloc.Path); err != nil {
		_ = os.RemoveAll(alloc.Path)
		_ = m.git(mirrorPath, "worktree", "prune")
	}
	_ = os.RemoveAll(filepath.Dir(alloc.Path)) // runID 目录空了顺带删
	zlog.Infof(ctx, "[Workspace] 工作区已释放: service=%s", alloc.Service)
}

// Resolve 按 service+env 查静态白名单：env 命中 refs 则用之，否则回退 default_ref（空为 HEAD）。
func Resolve(service, env string) (repoURL, ref string, err error) {
	cfg := conf.GetReactRuntimeConfig().Workspace
	for _, item := range cfg.Resolvers {
		if item.Service != service {
			continue
		}
		if env != "" {
			if r, ok := item.Refs[env]; ok && strings.TrimSpace(r) != "" {
				return item.RepoURL, strings.TrimSpace(r), nil
			}
		}
		return item.RepoURL, item.DefaultRef, nil
	}
	known := make([]string, 0, len(cfg.Resolvers))
	for _, item := range cfg.Resolvers {
		known = append(known, item.Service)
	}
	sort.Strings(known)
	return "", "", components.ErrorParamInvalid.Sprintf("service %q 未在 workspace.resolvers 白名单中，已知服务: %s", service, strings.Join(known, ", "))
}

// mirrorPath 由仓库 URL 推导 bare mirror 目录（同名仓库稳定映射）。
func (m *Manager) mirrorPath(repoURL string) string {
	name := strings.TrimSuffix(filepath.Base(strings.TrimRight(repoURL, "/")), ".git")
	if name == "" || !servicePattern.MatchString(name) {
		name = fmt.Sprintf("repo%d", len(repoURL))
	}
	return filepath.Join(m.mirrors, name+".git")
}

// ensureMirror 保证 bare mirror 存在并刷新到远端最新。
func (m *Manager) ensureMirror(repoURL string) (string, error) {
	mirror := m.mirrorPath(repoURL)
	if info, err := os.Stat(filepath.Join(mirror, "HEAD")); err == nil && !info.IsDir() {
		if err := m.git(mirror, "fetch", "--prune", "origin"); err != nil {
			return "", components.ErrorToolExecFailed.Sprintf("mirror 刷新失败(%s): %v", repoURL, err)
		}
		return mirror, nil
	}
	if err := os.MkdirAll(m.mirrors, 0o755); err != nil {
		return "", err
	}
	if err := m.git("", "clone", "--mirror", repoURL, mirror); err != nil {
		return "", components.ErrorToolExecFailed.Sprintf("mirror 克隆失败(%s): %v", repoURL, err)
	}
	return mirror, nil
}

// resolveCommit 在 mirror 内把 ref（分支/commit/tag，空为 HEAD）解析为完整 commit。
// ref 仅作为独立 argv 直传（已过 refPattern 字符校验），rev-parse 输出再按 40 位十六进制校验。
func (m *Manager) resolveCommit(mirrorPath, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref != "" && !refPattern.MatchString(ref) {
		return "", components.ErrorParamInvalid.Sprintf("ref 含非法字符: %q", ref)
	}
	args := make([]string, 0, 2)
	args = append(args, "rev-parse")
	if ref == "" {
		args = append(args, "HEAD")
	} else {
		args = append(args, ref)
	}
	out, err := m.gitOutput(mirrorPath, args...)
	if err != nil {
		return "", components.ErrorToolExecFailed.Sprintf("ref %q 解析失败: %v", ref, err)
	}
	// rev-parse 可能输出多行（如 tag 链），取首行并校验 commit 格式。
	firstLine := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if !commitPattern.MatchString(firstLine) {
		return "", components.ErrorToolExecFailed.Sprintf("ref %q 解析结果非法: %q", ref, firstLine)
	}
	return firstLine, nil
}

// git 在指定目录执行白名单内的 git 子命令。
func (m *Manager) git(dir string, args ...string) error {
	_, err := m.runGit(dir, args...)
	return err
}

func (m *Manager) gitOutput(dir string, args ...string) ([]byte, error) {
	return m.runGit(dir, args...)
}

// runGit 执行白名单内的 git 子命令。安全属性：
//   - 可执行文件是编译期常量 "git"，不经 shell（无 sh -c），argv 直传内核；
//   - 子命令必须在 gitSubcommands 白名单内；
//   - 全部动态参数（ref/commit/service/路径）在进入本函数前均已通过字符正则校验
//     （禁止空白与 shell 元字符），路径参数由 filepath.Join 生成且各段已校验。
func (m *Manager) runGit(dir string, args ...string) ([]byte, error) {
	if len(args) == 0 || !gitSubcommands[args[0]] {
		return nil, fmt.Errorf("git subcommand not allowed: %q", args[0])
	}
	timeout := conf.GetReactRuntimeConfig().Workspace.GitTimeoutSec
	cmd := exec.Command("git")
	cmd.Args = append(cmd.Args, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	timer := time.AfterFunc(time.Duration(timeout)*time.Second, func() { _ = cmd.Process.Kill() })
	defer timer.Stop()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// cleanupOrphans 清理上次进程遗留的 worktree 目录：worktree 生命周期不超过单个 run，
// 进程重启意味着全部 run 已终态（active 索引丢失），目录级全清即可。
func (m *Manager) cleanupOrphans() {
	if err := os.RemoveAll(m.root); err != nil {
		zlog.Warnf(nil, "[Workspace] 孤儿工作区清理失败(忽略): %v", err)
	}
}

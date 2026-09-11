// Package signal 提供进程退出钩子注册与执行的最小实现，替代原内部框架 server/signal 包。
package signal

import (
	"context"
	"sync"

	"react-base-service/golib/zlog"
)

var (
	mu    sync.Mutex
	hooks = map[string]func(ctx context.Context) error{}
)

// RegisterShutdown 注册进程退出钩子（按 name 去重）。
func RegisterShutdown(name string, fn func(ctx context.Context) error) {
	mu.Lock()
	defer mu.Unlock()
	hooks[name] = fn
}

// RunShutdownHooks 顺序执行全部退出钩子。
func RunShutdownHooks() error {
	mu.Lock()
	fns := make([]func(ctx context.Context) error, 0, len(hooks))
	for _, fn := range hooks {
		fns = append(fns, fn)
	}
	mu.Unlock()

	var firstErr error
	for _, fn := range fns {
		if err := fn(context.Background()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		zlog.Errorf(context.Background(), "shutdown hook error: %v", firstErr)
	}
	return firstErr
}

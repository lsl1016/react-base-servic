// Package http 提供 HTTP 服务启动的最小实现，替代原内部框架 server/http 包。
package http

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	signalsrv "react-base-service/golib/server/signal"

	"github.com/gin-gonic/gin"
)

// ServerConfig HTTP 服务配置。
type ServerConfig struct {
	Address string `yaml:"address"`
}

// Start 启动 HTTP 服务；收到退出信号后执行注册的 shutdown 钩子再优雅关闭。
func Start(engine *gin.Engine, cfg ServerConfig) error {
	addr := cfg.Address
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-quit:
	}

	_ = signalsrv.RunShutdownHooks()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

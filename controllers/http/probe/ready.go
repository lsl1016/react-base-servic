package probe

import (
	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

/*
Kubernetes使用就绪性探针（readiness probes）来实现探测服务是否准备好接收流量。

业务可以根据具体使用场景实现自己的探针，
在 Bootstrap 前通过 base.RegReadyProbe(probe.Ready) 注册探针即可。
*/
func Ready(ctx *gin.Context) {
	// 不打印本接口的日志，根据自己需求是否开启。
	zlog.SetNoLogFlag(ctx)
	// 业务逻辑

	// 业务逻辑判断没有问题返回：
	ctx.String(200, "success")

	// 如果业务逻辑判断资源未就绪，那么返回：
	// c.String(500, "fail")
}

package components

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"react-base-service/golib/base"
	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type DefaultRenderWithTrace struct {
	ErrNo   int         `json:"errNo"`
	ErrMsg  string      `json:"errMsg"`
	TraceId string      `json:"traceId"`
	Data    interface{} `json:"data"`
}

func RenderJsonSucc(ctx *gin.Context, data interface{}) {
	r := &DefaultRenderWithTrace{
		ErrNo:   0,
		ErrMsg:  "succ",
		TraceId: getTraceId(ctx),
		Data:    data,
	}
	setCommonHeader(ctx, 0)
	ctx.JSON(http.StatusOK, r)
}

func RenderJsonFail(ctx *gin.Context, err error) {
	var code int
	var msg string

	switch errors.Cause(err).(type) {
	case base.Error:
		code = errors.Cause(err).(base.Error).ErrNo
		msg = errors.Cause(err).(base.Error).ErrMsg
	case *base.Error:
		code = errors.Cause(err).(*base.Error).ErrNo
		msg = errors.Cause(err).(*base.Error).ErrMsg
	default:
		code = -1
		msg = errors.Cause(err).Error()
	}

	r := &DefaultRenderWithTrace{
		ErrNo:   code,
		ErrMsg:  msg,
		TraceId: getTraceId(ctx),
		Data:    gin.H{},
	}
	setCommonHeader(ctx, code)
	ctx.JSON(errorHTTPStatus(code), r)

	base.StackLogger(ctx, err)
}

func errorHTTPStatus(code int) int {
	switch code {
	case ErrorReactPlaygroundForbidden.ErrNo:
		return http.StatusForbidden
	default:
		return http.StatusOK
	}
}

func RenderSSEEvent(ctx *gin.Context, data interface{}) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(ctx.Writer, "data: %s\n\n", string(jsonBytes))
	ctx.Writer.Flush()
}

func RenderSSEDone(ctx *gin.Context) {
	fmt.Fprintf(ctx.Writer, "data: [DONE]\n\n")
	ctx.Writer.Flush()
}

func SetSSEHeaders(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("Transfer-Encoding", "chunked")
	ctx.Header("Trace-Id", getTraceId(ctx))
}

func setCommonHeader(ctx *gin.Context, code int) {
	ctx.Header("X-Err-No", strconv.Itoa(code))
	ctx.Header("Request-Id", zlog.GetRequestID(ctx))
	ctx.Header("Trace-Id", getTraceId(ctx))
}

func getTraceId(ctx *gin.Context) string {
	if logId := zlog.GetLogID(ctx); logId != "" {
		return logId
	}
	if requestId := zlog.GetRequestID(ctx); requestId != "" {
		return requestId
	}
	return "unknown"
}

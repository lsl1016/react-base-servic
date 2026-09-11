// Package base 提供错误码、HTTP 客户端与 MySQL 初始化的最小实现，替代原内部框架 base 包。
package base

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"react-base-service/golib/zlog"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Error 业务错误（ErrNo + ErrMsg），实现 error 接口。
type Error struct {
	ErrNo  int    `json:"errNo"`
	ErrMsg string `json:"errMsg"`
}

func (e Error) Error() string {
	return fmt.Sprintf("errNo=%d errMsg=%s", e.ErrNo, e.ErrMsg)
}

// Sprintf 以 ErrMsg 为模板格式化并返回保留 ErrNo 的错误。
func (e Error) Sprintf(args ...interface{}) error {
	return errors.WithStack(Error{ErrNo: e.ErrNo, ErrMsg: fmt.Sprintf(e.ErrMsg, args...)})
}

// Wrap 包装底层错误并保留 ErrNo，错误信息追加底层原因。
func (e Error) Wrap(err error) error {
	return errors.WithStack(Error{ErrNo: e.ErrNo, ErrMsg: fmt.Sprintf("%s: %v", e.ErrMsg, err)})
}

// PprofConfig pprof 开关配置。
type PprofConfig struct {
	Enable bool `yaml:"enable"`
}

// ApiClient 外部 HTTP 服务客户端配置。
type ApiClient struct {
	AppKey          string `yaml:"appKey"`
	AppSecret       string `yaml:"appSecret"`
	Domain          string `yaml:"domain"`
	Host            string `yaml:"host"`
	HttpStat        bool   `yaml:"httpStat"`
	IdleConnTimeout string `yaml:"idleConnTimeout"`
	MaxIdleConns    int    `yaml:"maxIdleConns"`
	Retry           int    `yaml:"retry"`
	Service         string `yaml:"service"`
	Timeout         string `yaml:"timeout"`
}

// HttpRequestOptions HttpPost 请求参数。
type HttpRequestOptions struct {
	RequestBody interface{}
	Encode      func(v interface{}) ([]byte, error)
	ContentType string
	Headers     map[string]string
}

// EncodeJson JSON 编码器。
func EncodeJson(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// HttpResponse HttpPost 响应。
type HttpResponse struct {
	HttpCode int
	Response []byte
}

// HttpPost 以 Domain 为基址发起 POST 请求。
func (c *ApiClient) HttpPost(ctx *gin.Context, path string, opt HttpRequestOptions) (*HttpResponse, error) {
	return c.doRequest(ctx, http.MethodPost, path, opt)
}

func (c *ApiClient) doRequest(ctx *gin.Context, method, path string, opt HttpRequestOptions) (*HttpResponse, error) {
	if strings.TrimSpace(c.Domain) == "" {
		return nil, errors.New("api client domain not configured")
	}

	var body []byte
	if opt.RequestBody != nil {
		encode := opt.Encode
		if encode == nil {
			encode = EncodeJson
		}
		data, err := encode(opt.RequestBody)
		if err != nil {
			return nil, errors.Wrap(err, "encode request body")
		}
		body = data
	}

	timeout := 30 * time.Second
	if d, err := time.ParseDuration(c.Timeout); err == nil && d > 0 {
		timeout = d
	}
	client := &http.Client{Timeout: timeout}

	url := strings.TrimRight(c.Domain, "/") + path
	req, err := http.NewRequestWithContext(requestContextOrBackground(ctx), method, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Wrap(err, "build request")
	}
	if opt.ContentType != "" {
		req.Header.Set("Content-Type", opt.ContentType)
	}
	for k, v := range opt.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request")
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response")
	}
	return &HttpResponse{HttpCode: resp.StatusCode, Response: respBody}, nil
}

func requestContextOrBackground(ctx *gin.Context) context.Context {
	if ctx == nil || ctx.Request == nil || ctx.Request.Context() == nil {
		return context.Background()
	}
	return ctx.Request.Context()
}

// StackLogger 以错误级别打印错误与调用栈。
func StackLogger(ctx *gin.Context, err error) {
	zlog.Errorf(ctx, "[stack] %v", err)
	buf := make([]byte, 8192)
	n := runtime.Stack(buf, false)
	for _, line := range strings.Split(string(buf[:n]), "\n") {
		if strings.TrimSpace(line) != "" {
			zlog.Errorf(ctx, "[stack] %s", strings.TrimSpace(line))
		}
	}
}

// RenderJsonAbort 输出错误响应并终止后续处理。
func RenderJsonAbort(c *gin.Context, err error) {
	var (
		code int
		msg  string
	)
	switch cause := errors.Cause(err).(type) {
	case Error:
		code, msg = cause.ErrNo, cause.ErrMsg
	case *Error:
		code, msg = cause.ErrNo, cause.ErrMsg
	default:
		code, msg = -1, fmt.Sprintf("%v", err)
	}
	c.Header("X-Err-No", strconv.Itoa(code))
	c.AbortWithStatusJSON(http.StatusOK, gin.H{"errNo": code, "errMsg": msg, "data": gin.H{}})
}

// readyProbes 就绪探针注册表（本地实现仅存储，不自动挂载路由）。
var readyProbes []func(ctx *gin.Context)

// RegReadyProbe 注册就绪探针。
func RegReadyProbe(probe func(ctx *gin.Context)) {
	readyProbes = append(readyProbes, probe)
}

// MysqlConf MySQL 连接配置。
type MysqlConf struct {
	Addr            string `yaml:"addr"`
	ConnMaxLifeTime string `yaml:"connMaxLifeTime"`
	ConnTimeOut     string `yaml:"connTimeOut"`
	Database        string `yaml:"database"`
	MaxIdleTime     string `yaml:"maxIdleTime"`
	MaxIdleConns    int    `yaml:"maxidleconns"`
	MaxOpenConns    int    `yaml:"maxopenconns"`
	ReadTimeOut     string `yaml:"readTimeOut"`
	Service         string `yaml:"service"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	WriteTimeOut    string `yaml:"writeTimeOut"`
}

// InitMysqlClient 建立 GORM MySQL 连接。
func InitMysqlClient(cfg MysqlConf) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Addr, cfg.Database)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, errors.Wrap(err, "open mysql")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "mysql raw db")
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if d, err := time.ParseDuration(cfg.ConnMaxLifeTime); err == nil && d > 0 {
		sqlDB.SetConnMaxLifetime(d)
	}
	if d, err := time.ParseDuration(cfg.MaxIdleTime); err == nil && d > 0 {
		sqlDB.SetConnMaxIdleTime(d)
	}
	return db, sqlDB.Ping()
}

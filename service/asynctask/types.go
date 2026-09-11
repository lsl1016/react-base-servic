package asynctask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	ExecutionStatusProcessing = "processing"
	ExecutionStatusSucceeded  = "succeeded"
	ExecutionStatusFailed     = "failed"
)

// TaskSubmission 是异步 Tool 已成功执行后的提交上下文；Provider 只据此识别任务，不负责再次提交。
type TaskSubmission struct {
	ToolName     string
	SubmitInput  json.RawMessage
	SubmitResult json.RawMessage
	UserName     string
}

// TaskIdentity 是第三方任务在本服务中的稳定身份。
type TaskIdentity struct {
	TaskKey  string
	TaskInfo json.RawMessage
}

// TaskObservation 是 Provider 从 MQ、接口或其他来源获得的统一任务状态观测。
type TaskObservation struct {
	SchedulerType   string
	TaskKey         string
	ExecutionStatus string
	ProviderStatus  string
	Progress        *int
	ErrorMessage    string
	EventID         string
	Sequence        int64
	ObservedAt      time.Time
	Source          string
}

// TaskSnapshot 是 Provider 主动对账时需要的最小任务快照。
type TaskSnapshot struct {
	ID              uint
	TaskKey         string
	TaskInfo        json.RawMessage
	ExecutionStatus string
	RetryCount      int
	LastObservedAt  *time.Time
}

// ProviderRuntime 由公共运行时实现；Provider 不直接操作数据库。
type ProviderRuntime interface {
	EmitObservation(ctx context.Context, observation TaskObservation) error
	ClaimTasks(ctx context.Context, schedulerType, owner string, staleBefore time.Time, limit int, lease time.Duration) ([]TaskSnapshot, error)
	ScheduleRetry(ctx context.Context, taskID uint, owner string, nextSyncAt time.Time, cause error) error
	ReleaseTask(ctx context.Context, taskID uint, owner string, nextSyncAt time.Time) error
}

// AvailabilityProvider 可选声明 Provider 当前是否已配置启用。
type AvailabilityProvider interface {
	Available() bool
}

// SchedulerProvider 封装一个调度系统的任务识别及完整状态同步逻辑。
type SchedulerProvider interface {
	Type() string
	IdentifyTask(ctx context.Context, submission TaskSubmission) (TaskIdentity, error)
	Start(ctx context.Context, engine *gin.Engine, runtime ProviderRuntime) error
	Stop()
}

var registry = struct {
	sync.RWMutex
	providers map[string]SchedulerProvider
}{providers: make(map[string]SchedulerProvider)}

// Register 注册调度系统 Provider；同一类型重复注册会直接失败，避免实现被静默覆盖。
func Register(provider SchedulerProvider) {
	if provider == nil {
		panic("async task provider is nil")
	}
	providerType := strings.ToLower(strings.TrimSpace(provider.Type()))
	if providerType == "" {
		panic("async task provider type is empty")
	}
	registry.Lock()
	defer registry.Unlock()
	if _, exists := registry.providers[providerType]; exists {
		panic(fmt.Sprintf("async task provider already registered: %s", providerType))
	}
	registry.providers[providerType] = provider
}

// Get 返回指定调度系统 Provider。
func Get(providerType string) (SchedulerProvider, bool) {
	registry.RLock()
	defer registry.RUnlock()
	provider, ok := registry.providers[strings.ToLower(strings.TrimSpace(providerType))]
	return provider, ok
}

func listProviders() []SchedulerProvider {
	registry.RLock()
	defer registry.RUnlock()
	providers := make([]SchedulerProvider, 0, len(registry.providers))
	for _, provider := range registry.providers {
		providers = append(providers, provider)
	}
	return providers
}

// IsTerminalExecutionStatus 判断第三方规范化执行状态是否已经终结。
func IsTerminalExecutionStatus(status string) bool {
	switch status {
	case ExecutionStatusSucceeded, ExecutionStatusFailed:
		return true
	default:
		return false
	}
}

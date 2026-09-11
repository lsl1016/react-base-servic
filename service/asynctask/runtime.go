package asynctask

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"react-base-service/conf"
	model "react-base-service/models/llm"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
)

// ManagedTaskFields 是 AsyncTaskManager 在 Tool 成功后交给 ReAct 任务记录的托管字段。
type ManagedTaskFields struct {
	SchedulerType   string
	TaskKey         string
	TaskInfo        string
	ExecutionStatus string
	NextSyncAt      *time.Time
}

// IdentifySubmission 根据 Tool 配置绑定的调度系统识别任务；失败由调用方降级为旧提醒模式。
func IdentifySubmission(ctx context.Context, schedulerType string, submission TaskSubmission) (ManagedTaskFields, error) {
	providerType := strings.ToLower(strings.TrimSpace(schedulerType))
	if !conf.CustomConf.AsyncTask.Enabled {
		return ManagedTaskFields{}, fmt.Errorf("async task framework is disabled")
	}
	provider, ok := Get(providerType)
	if !ok {
		return ManagedTaskFields{}, fmt.Errorf("async task provider not registered: %s", providerType)
	}
	if availability, ok := provider.(AvailabilityProvider); ok && !availability.Available() {
		return ManagedTaskFields{}, fmt.Errorf("async task provider is disabled: %s", providerType)
	}
	identity, err := provider.IdentifyTask(ctx, submission)
	if err != nil {
		return ManagedTaskFields{}, err
	}
	identity.TaskKey = strings.TrimSpace(identity.TaskKey)
	if identity.TaskKey == "" || len(identity.TaskInfo) == 0 || string(identity.TaskInfo) == "null" {
		return ManagedTaskFields{}, fmt.Errorf("provider %s returned empty task identity", providerType)
	}
	now := time.Now()
	return ManagedTaskFields{
		SchedulerType:   providerType,
		TaskKey:         identity.TaskKey,
		TaskInfo:        string(identity.TaskInfo),
		ExecutionStatus: ExecutionStatusProcessing,
		NextSyncAt:      &now,
	}, nil
}

type runtime struct{}

func (runtime) EmitObservation(ctx context.Context, observation TaskObservation) error {
	observation.SchedulerType = strings.ToLower(strings.TrimSpace(observation.SchedulerType))
	observation.TaskKey = strings.TrimSpace(observation.TaskKey)
	if observation.SchedulerType == "" || observation.TaskKey == "" {
		return fmt.Errorf("task observation schedulerType and taskKey are required")
	}
	if !isValidExecutionStatus(observation.ExecutionStatus) {
		return fmt.Errorf("invalid async task execution status: %s", observation.ExecutionStatus)
	}
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now()
	}
	_, err := model.ApplyReactAsyncTaskObservation(ctx, observation.SchedulerType, observation.TaskKey, observation.ExecutionStatus, observation.ProviderStatus, observation.Progress, observation.ErrorMessage, observation.EventID, observation.Sequence, observation.ObservedAt)
	return err
}

func (runtime) ClaimTasks(ctx context.Context, schedulerType, owner string, staleBefore time.Time, limit int, lease time.Duration) ([]TaskSnapshot, error) {
	tasks, err := model.ClaimReactAsyncTasks(ctx, schedulerType, owner, staleBefore, limit, lease)
	if err != nil {
		return nil, err
	}
	result := make([]TaskSnapshot, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, TaskSnapshot{
			ID:              task.ID,
			TaskKey:         task.TaskKey,
			TaskInfo:        []byte(task.TaskInfo),
			ExecutionStatus: task.ExecutionStatus,
			RetryCount:      task.RetryCount,
			LastObservedAt:  task.LastObservedAt,
		})
	}
	return result, nil
}

func (runtime) ScheduleRetry(ctx context.Context, taskID uint, owner string, nextSyncAt time.Time, cause error) error {
	return model.ScheduleReactAsyncTaskRetry(ctx, taskID, owner, nextSyncAt, cause)
}

func (runtime) ReleaseTask(ctx context.Context, taskID uint, owner string, nextSyncAt time.Time) error {
	return model.ReleaseReactAsyncTaskLease(ctx, taskID, owner, nextSyncAt)
}

func isValidExecutionStatus(status string) bool {
	switch status {
	case ExecutionStatusProcessing, ExecutionStatusSucceeded, ExecutionStatusFailed:
		return true
	default:
		return false
	}
}

var lifecycle = struct {
	sync.Mutex
	cancel  context.CancelFunc
	running bool
}{}

// Start 启动已注册 Provider。空注册表或总开关关闭时安全返回。
func Start(engine *gin.Engine) {
	if !conf.CustomConf.AsyncTask.Enabled {
		return
	}
	lifecycle.Lock()
	defer lifecycle.Unlock()
	if lifecycle.running {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := make([]SchedulerProvider, 0)
	for _, provider := range listProviders() {
		if availability, ok := provider.(AvailabilityProvider); ok && !availability.Available() {
			continue
		}
		if err := provider.Start(ctx, engine, runtime{}); err != nil {
			zlog.Errorf(nil, "[AsyncTask.Start] 启动 Provider 失败: provider=%s err=%v", provider.Type(), err)
			continue
		}
		started = append(started, provider)
	}
	if len(started) == 0 {
		cancel()
		return
	}
	lifecycle.cancel = cancel
	lifecycle.running = true
	zlog.Infof(nil, "[AsyncTask.Start] 异步任务同步框架已启动: providers=%d", len(started))
}

// Stop 停止 Provider 后台组件。
func Stop() {
	lifecycle.Lock()
	defer lifecycle.Unlock()
	if !lifecycle.running {
		return
	}
	if lifecycle.cancel != nil {
		lifecycle.cancel()
	}
	for _, provider := range listProviders() {
		provider.Stop()
	}
	lifecycle.cancel = nil
	lifecycle.running = false
}

package model

import (
	"errors"
	"time"

	"react-base-service/components"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	ReactAsyncTaskStatePending  = "pending"
	ReactAsyncTaskStateResolved = "resolved"
	ReactAsyncTaskStateExpired  = "expired"

	ReactAsyncTaskExecutionProcessing = "processing"
	ReactAsyncTaskExecutionSucceeded  = "succeeded"
	ReactAsyncTaskExecutionFailed     = "failed"

	reactAsyncTaskUnsetProgress = -1
	reactAsyncTaskUnsetDateTime = "1970-01-01 00:00:00"
)

// ReactAsyncTask 记录异步提交型工具已受理但结果未确认的任务；
// 后续 run 构建上下文时按 session 注入提醒，由模型自行查询并通过 resolve_async_task 标记完结。
// submit_input/submit_result 存全文快照（超长才截），历史被压缩后仍可关联任务业务语义，
// 提醒里被截断的内容可通过 get_async_task 回读完整记录。
type ReactAsyncTask struct {
	ID              uint       `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	SessionID       string     `json:"sessionId" gorm:"column:session_id;not null"`
	RunID           string     `json:"runId" gorm:"column:run_id;not null"`
	ToolUseID       string     `json:"toolUseId" gorm:"column:tool_use_id;not null"`
	ToolName        string     `json:"toolName" gorm:"column:tool_name;not null;default:''"`
	SubmitInput     string     `json:"submitInput" gorm:"column:submit_input;type:mediumtext"`
	SubmitResult    string     `json:"submitResult" gorm:"column:submit_result;type:mediumtext"`
	AsyncHint       string     `json:"asyncHint" gorm:"column:async_hint;not null;default:''"`
	SchedulerType   string     `json:"schedulerType" gorm:"column:scheduler_type;not null;default:''"`
	TaskKey         string     `json:"taskKey" gorm:"column:task_key;not null;default:''"`
	TaskInfo        string     `json:"taskInfo" gorm:"column:task_info;type:json"`
	State           string     `json:"state" gorm:"column:state;not null;default:'pending'"`
	ExecutionStatus string     `json:"executionStatus" gorm:"column:execution_status;not null;default:''"`
	ProviderStatus  string     `json:"providerStatus" gorm:"column:provider_status;not null;default:''"`
	Progress        *int       `json:"progress,omitempty" gorm:"column:progress;not null;default:-1"`
	ErrorMessage    string     `json:"errorMessage" gorm:"column:error_message;type:text"`
	LastEventID     string     `json:"lastEventId" gorm:"column:last_event_id;not null;default:''"`
	LastSequence    int64      `json:"lastSequence" gorm:"column:last_sequence;not null;default:0"`
	LastObservedAt  *time.Time `json:"lastObservedAt,omitempty" gorm:"column:last_observed_at;not null;default:'1970-01-01 00:00:00'"`
	CompletedAt     *time.Time `json:"completedAt,omitempty" gorm:"column:completed_at;not null;default:'1970-01-01 00:00:00'"`
	NextSyncAt      *time.Time `json:"nextSyncAt,omitempty" gorm:"column:next_sync_at;not null;default:'1970-01-01 00:00:00'"`
	RetryCount      int        `json:"retryCount" gorm:"column:retry_count;not null;default:0"`
	LeaseOwner      string     `json:"leaseOwner" gorm:"column:lease_owner;not null;default:''"`
	LeaseUntil      *time.Time `json:"leaseUntil,omitempty" gorm:"column:lease_until;not null;default:'1970-01-01 00:00:00'"`
	LastSyncError   string     `json:"lastSyncError" gorm:"column:last_sync_error;type:text"`
	ResolveStatus   string     `json:"resolveStatus" gorm:"column:resolve_status;not null;default:''"`
	ExpireAt        time.Time  `json:"expireAt" gorm:"column:expire_at;not null"`
	CreatedAt       time.Time  `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt       time.Time  `json:"updatedAt" gorm:"column:updated_at"`
}

func (r *ReactAsyncTask) TableName() string {
	return "tblLlmReactAsyncTask"
}

// AfterFind 将数据库哨兵值还原为业务层的未设置状态。
func (r *ReactAsyncTask) AfterFind(_ *gorm.DB) error {
	if r.Progress != nil && *r.Progress == reactAsyncTaskUnsetProgress {
		r.Progress = nil
	}
	if isReactAsyncTaskUnsetTime(r.LastObservedAt) {
		r.LastObservedAt = nil
	}
	if isReactAsyncTaskUnsetTime(r.CompletedAt) {
		r.CompletedAt = nil
	}
	if isReactAsyncTaskUnsetTime(r.NextSyncAt) {
		r.NextSyncAt = nil
	}
	if isReactAsyncTaskUnsetTime(r.LeaseUntil) {
		r.LeaseUntil = nil
	}
	return nil
}

func isReactAsyncTaskUnsetTime(value *time.Time) bool {
	return value != nil && value.Year() == 1970 && value.Month() == time.January && value.Day() == 1 &&
		value.Hour() == 0 && value.Minute() == 0 && value.Second() == 0
}

// reactAsyncTaskDefaultFields 返回应交由数据库写入默认哨兵值的字段。
func reactAsyncTaskDefaultFields(task *ReactAsyncTask) []string {
	fields := make([]string, 0, 5)
	if task.Progress == nil {
		fields = append(fields, "Progress")
	}
	if task.LastObservedAt == nil {
		fields = append(fields, "LastObservedAt")
	}
	if task.CompletedAt == nil {
		fields = append(fields, "CompletedAt")
	}
	if task.NextSyncAt == nil {
		fields = append(fields, "NextSyncAt")
	}
	if task.LeaseUntil == nil {
		fields = append(fields, "LeaseUntil")
	}
	return fields
}

func CreateReactAsyncTask(ctx *gin.Context, task *ReactAsyncTask) error {
	db := helpers.MysqlClientLLM.Model(&ReactAsyncTask{}).WithContext(ctx)
	if fields := reactAsyncTaskDefaultFields(task); len(fields) > 0 {
		db = db.Omit(fields...)
	}
	err := db.Create(task).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

// ListPendingReactAsyncTasks 按提交时间倒序返回 session 内未完结且未过期的异步任务。
func ListPendingReactAsyncTasks(ctx *gin.Context, sessionID string, limit int) ([]ReactAsyncTask, error) {
	if limit <= 0 {
		limit = 10
	}
	var tasks []ReactAsyncTask
	err := helpers.MysqlClientLLM.Model(&ReactAsyncTask{}).WithContext(ctx).
		Where("session_id = ? AND state = ? AND expire_at > ?", sessionID, ReactAsyncTaskStatePending, time.Now()).
		Order("id DESC").Limit(limit).Find(&tasks).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return tasks, nil
}

// GetReactAsyncTaskByToolUseID 按会话隔离读取单个异步任务完整记录，不过滤状态，供 get_async_task 回读。
func GetReactAsyncTaskByToolUseID(ctx *gin.Context, sessionID, toolUseID string) (*ReactAsyncTask, error) {
	var task ReactAsyncTask
	err := helpers.MysqlClientLLM.Model(&ReactAsyncTask{}).WithContext(ctx).
		Where("session_id = ? AND tool_use_id = ?", sessionID, toolUseID).First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &task, nil
}

// ResolveReactAsyncTask 将 pending 任务置为 resolved；返回影响行数用于区分"任务不存在或已完结"。
func ResolveReactAsyncTask(ctx *gin.Context, sessionID, toolUseID, resolveStatus string) (int64, error) {
	result := helpers.MysqlClientLLM.Model(&ReactAsyncTask{}).WithContext(ctx).
		Where("session_id = ? AND tool_use_id = ? AND state = ?", sessionID, toolUseID, ReactAsyncTaskStatePending).
		Updates(map[string]interface{}{"state": ReactAsyncTaskStateResolved, "resolve_status": resolveStatus})
	if result.Error != nil {
		return 0, components.ErrorDbUpdate.Wrap(result.Error)
	}
	return result.RowsAffected, nil
}

// ExpireReactAsyncTasks 惰性过期：在读取 pending 列表前把超时任务置为 expired，无需定时任务。
func ExpireReactAsyncTasks(ctx *gin.Context, sessionID string) error {
	err := helpers.MysqlClientLLM.Model(&ReactAsyncTask{}).WithContext(ctx).
		Where("session_id = ? AND state = ? AND expire_at <= ?", sessionID, ReactAsyncTaskStatePending, time.Now()).
		Update("state", ReactAsyncTaskStateExpired).Error
	if err != nil {
		return components.ErrorDbUpdate.Wrap(err)
	}
	return nil
}

package model

import (
	"context"
	"encoding/json"
	"time"

	"react-base-service/components"
	"react-base-service/helpers"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExpireReactAsyncTasksBySession 将指定会话中已超时的待处理任务统一过期。
func ExpireReactAsyncTasksBySession(ctx context.Context, sessionID string) error {
	if err := helpers.MysqlClientLLM.WithContext(ctx).Model(&ReactAsyncTask{}).
		Where("session_id = ? AND state = ? AND expire_at <= ?", sessionID, ReactAsyncTaskStatePending, time.Now()).
		Update("state", ReactAsyncTaskStateExpired).Error; err != nil {
		return components.ErrorDbUpdate.Wrap(err)
	}
	return nil
}

// ListReadyReactAsyncTasksBySessionPage 分页返回会话下第三方已终态但本地尚未处理的任务。
func ListReadyReactAsyncTasksBySessionPage(ctx context.Context, sessionID string, afterID uint, limit int) ([]ReactAsyncTask, error) {
	if limit <= 0 {
		limit = 100
	}
	var tasks []ReactAsyncTask
	query := helpers.MysqlClientLLM.WithContext(ctx).Model(&ReactAsyncTask{}).
		Where("session_id = ? AND state = ? AND execution_status IN ?", sessionID, ReactAsyncTaskStatePending, []string{ReactAsyncTaskExecutionSucceeded, ReactAsyncTaskExecutionFailed})
	if afterID > 0 {
		query = query.Where("id > ?", afterID)
	}
	if err := query.Order("id ASC").Limit(limit).Find(&tasks).Error; err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return tasks, nil
}

// HasProcessingReactAsyncTasks 判断当前会话是否仍有由 Provider 跟踪的执行中任务。
func HasProcessingReactAsyncTasks(ctx context.Context, sessionID string) (bool, error) {
	var count int64
	if err := helpers.MysqlClientLLM.WithContext(ctx).Model(&ReactAsyncTask{}).
		Where("session_id = ? AND state = ? AND execution_status = ?", sessionID, ReactAsyncTaskStatePending, ReactAsyncTaskExecutionProcessing).
		Limit(1).Count(&count).Error; err != nil {
		return false, components.ErrorDbSelect.Wrap(err)
	}
	return count > 0, nil
}

// ClaimReactAsyncTasks 抢占长期未更新且需要主动对账的任务，支持多实例并发运行。
func ClaimReactAsyncTasks(ctx context.Context, schedulerType, owner string, staleBefore time.Time, limit int, lease time.Duration) ([]ReactAsyncTask, error) {
	if limit <= 0 {
		limit = 100
	}
	now := time.Now()
	leaseUntil := now.Add(lease)
	var claimed []ReactAsyncTask
	err := helpers.MysqlClientLLM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidates []ReactAsyncTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("scheduler_type = ? AND execution_status = ?", schedulerType, ReactAsyncTaskExecutionProcessing).
			Where("(next_sync_at IS NULL OR next_sync_at <= ?) AND (lease_until IS NULL OR lease_until < ?)", now, now).
			Where("last_observed_at IS NULL OR last_observed_at <= ?", staleBefore).
			Order("id ASC").Limit(limit).Find(&candidates).Error; err != nil {
			return err
		}
		for index := range candidates {
			result := tx.Model(&ReactAsyncTask{}).
				Where("id = ? AND execution_status = ? AND (lease_until IS NULL OR lease_until < ?)", candidates[index].ID, ReactAsyncTaskExecutionProcessing, now).
				Updates(map[string]interface{}{"lease_owner": owner, "lease_until": &leaseUntil})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				candidates[index].LeaseOwner = owner
				candidates[index].LeaseUntil = &leaseUntil
				claimed = append(claimed, candidates[index])
			}
		}
		return nil
	})
	if err != nil {
		return nil, components.ErrorDbUpdate.Wrap(err)
	}
	return claimed, nil
}

// ApplyReactAsyncTaskObservation 将一次状态观测应用到同一第三方任务的执行中记录；相同终态允许补充详情。
func ApplyReactAsyncTaskObservation(ctx context.Context, schedulerType, taskKey, executionStatus, providerStatus string, progress *int, errorMessage, eventID string, sequence int64, observedAt time.Time) (int64, error) {
	var affected int64
	err := helpers.MysqlClientLLM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tasks []ReactAsyncTask
		eligibleStatuses := []string{ReactAsyncTaskExecutionProcessing}
		if isTerminalReactAsyncExecution(executionStatus) {
			eligibleStatuses = append(eligibleStatuses, executionStatus)
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("scheduler_type = ? AND task_key = ? AND execution_status IN ?", schedulerType, taskKey, eligibleStatuses).
			Find(&tasks).Error; err != nil {
			return err
		}
		for index := range tasks {
			task := &tasks[index]
			if task.LastEventID != "" && eventID != "" && task.LastEventID == eventID {
				continue
			}
			if sequence > 0 && task.LastSequence > 0 && sequence <= task.LastSequence {
				continue
			}
			if task.LastObservedAt != nil && observedAt.Before(*task.LastObservedAt) {
				continue
			}
			if isTerminalReactAsyncExecution(task.ExecutionStatus) && executionStatus != task.ExecutionStatus {
				continue
			}
			updates := map[string]interface{}{
				"execution_status": executionStatus,
				"provider_status":  providerStatus,
				"error_message":    errorMessage,
				"last_observed_at": &observedAt,
				"retry_count":      0,
				"last_sync_error":  "",
				"lease_owner":      "",
				"lease_until":      reactAsyncTaskUnsetDateTime,
			}
			if progress != nil {
				updates["progress"] = *progress
			}
			if eventID != "" {
				updates["last_event_id"] = eventID
			}
			if sequence > 0 {
				updates["last_sequence"] = sequence
			}
			if isTerminalReactAsyncExecution(executionStatus) {
				updates["completed_at"] = &observedAt
				updates["next_sync_at"] = reactAsyncTaskUnsetDateTime
			}
			result := tx.Model(&ReactAsyncTask{}).Where("id = ?", task.ID).Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			affected += result.RowsAffected
		}
		return nil
	})
	if err != nil {
		return 0, components.ErrorDbUpdate.Wrap(err)
	}
	return affected, nil
}

func isTerminalReactAsyncExecution(status string) bool {
	switch status {
	case ReactAsyncTaskExecutionSucceeded, ReactAsyncTaskExecutionFailed:
		return true
	default:
		return false
	}
}

func ScheduleReactAsyncTaskRetry(ctx context.Context, taskID uint, owner string, nextSyncAt time.Time, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	result := helpers.MysqlClientLLM.WithContext(ctx).Model(&ReactAsyncTask{}).
		Where("id = ? AND lease_owner = ?", taskID, owner).
		Updates(map[string]interface{}{
			"retry_count":     gorm.Expr("retry_count + 1"),
			"last_sync_error": message,
			"next_sync_at":    &nextSyncAt,
			"lease_owner":     "",
			"lease_until":     reactAsyncTaskUnsetDateTime,
		})
	if result.Error != nil {
		return components.ErrorDbUpdate.Wrap(result.Error)
	}
	return nil
}

func ReleaseReactAsyncTaskLease(ctx context.Context, taskID uint, owner string, nextSyncAt time.Time) error {
	result := helpers.MysqlClientLLM.WithContext(ctx).Model(&ReactAsyncTask{}).
		Where("id = ? AND lease_owner = ?", taskID, owner).
		Updates(map[string]interface{}{"next_sync_at": &nextSyncAt, "lease_owner": "", "lease_until": reactAsyncTaskUnsetDateTime})
	if result.Error != nil {
		return components.ErrorDbUpdate.Wrap(result.Error)
	}
	return nil
}

// DecodeReactAsyncTaskInfo 将任务信息 JSON 解码到 Provider 自己的结构中。
func DecodeReactAsyncTaskInfo(task ReactAsyncTask, target interface{}) error {
	return json.Unmarshal([]byte(task.TaskInfo), target)
}

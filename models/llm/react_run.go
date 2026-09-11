package model

import (
	"context"
	"errors"
	"time"

	"react-base-service/components"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	ReactRunStateRunning              = "running"
	ReactRunStateWaitingClientMessage = "waiting_client_message"
	ReactRunStateCancelling           = "cancelling"
	ReactRunStateFinished             = "finished"
	ReactRunStateError                = "error"
	ReactRunStateCancelled            = "cancelled"
	ReactRunStateExpired              = "expired"
)

type ReactRun struct {
	ID                      uint      `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	RunID                   string    `json:"runId" gorm:"column:run_id;not null"`
	SessionID               string    `json:"sessionId" gorm:"column:session_id;not null"`
	UserName                string    `json:"userName" gorm:"column:user_name;not null"`
	CallerKey               string    `json:"callerKey" gorm:"column:caller_key;not null"`
	RouteValues             string    `json:"routeValues" gorm:"column:route_values;not null;default:''"`
	State                   string    `json:"state" gorm:"column:state;not null"`
	StepIndex               int       `json:"stepIndex" gorm:"column:step_index;not null;default:0"`
	MaxSteps                int       `json:"maxSteps" gorm:"column:max_steps;not null"`
	ModelKey                string    `json:"modelKey" gorm:"column:model_key"`
	ModelVersion            string    `json:"modelVersion" gorm:"column:model_version"`
	ApiKey                  string    `json:"apiKey" gorm:"column:api_key"`
	ControlContextJSON      string    `json:"controlContextJson" gorm:"column:control_context_json;type:mediumtext"`
	LLMContextJSON          string    `json:"llmContextJson" gorm:"column:llm_context_json;type:mediumtext"`
	ToolIndexSnapshotJSON   string    `json:"toolIndexSnapshotJson" gorm:"column:tool_index_snapshot_json;type:mediumtext"`
	ActiveToolIDs           string    `json:"activeToolIds" gorm:"column:active_tool_ids;type:text"`
	ActiveToolDefsJSON      string    `json:"activeToolDefsJson" gorm:"column:active_tool_defs_json;type:mediumtext"`
	SkillsIndexSnapshotJSON string    `json:"skillsIndexSnapshotJson" gorm:"column:skills_index_snapshot_json;type:mediumtext"`
	LoadedSkillIDs          string    `json:"loadedSkillIds" gorm:"column:loaded_skill_ids;type:text"`
	PendingToolUseIDs       string    `json:"pendingToolUseIds" gorm:"column:pending_tool_use_ids;type:text"`
	TodoStateJSON           string    `json:"todoStateJson" gorm:"column:todo_state_json;type:text"`
	TotalInputTokens        int       `json:"totalInputTokens" gorm:"column:total_input_tokens;not null;default:0"`
	TotalOutputTokens       int       `json:"totalOutputTokens" gorm:"column:total_output_tokens;not null;default:0"`
	LastInputTokens         int       `json:"lastInputTokens" gorm:"column:last_input_tokens;not null;default:0"`
	LastOutputTokens        int       `json:"lastOutputTokens" gorm:"column:last_output_tokens;not null;default:0"`
	CacheReadTokens         int       `json:"cacheReadTokens" gorm:"column:cache_read_tokens;not null;default:0"`
	CacheCreateTokens       int       `json:"cacheCreateTokens" gorm:"column:cache_create_tokens;not null;default:0"`
	ErrorMessage            string    `json:"errorMessage" gorm:"column:error_message;type:text"`
	CreatedAt               time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt               time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (r *ReactRun) TableName() string {
	return "tblLlmReactRun"
}

func CreateReactRun(ctx *gin.Context, run *ReactRun) error {
	return CreateReactRunWithDB(ctx, helpers.MysqlClientLLM, run)
}

func CreateReactRunWithDB(ctx *gin.Context, db *gorm.DB, run *ReactRun) error {
	if run.State == "" {
		run.State = ReactRunStateRunning
	}
	err := db.Model(&ReactRun{}).WithContext(ctx).Create(run).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

func GetReactRunByRunID(ctx *gin.Context, runID string) (*ReactRun, error) {
	var run ReactRun
	err := helpers.MysqlClientLLM.Model(&ReactRun{}).WithContext(ctx).
		Where("run_id = ?", runID).First(&run).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &run, nil
}

func UpdateReactRunByRunID(ctx *gin.Context, runID string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	tx := helpers.MysqlClientLLM.Model(&ReactRun{}).WithContext(ctx).
		Where("run_id = ?", runID).
		Updates(updates)
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func ExpireReactRunsByRunIDs(ctx context.Context, runIDs []string) error {
	if len(runIDs) == 0 {
		return nil
	}
	tx := helpers.MysqlClientLLM.Model(&ReactRun{}).WithContext(ctx).
		Where("run_id IN ? AND state IN ?", runIDs, []string{
			ReactRunStateRunning,
			ReactRunStateWaitingClientMessage,
			ReactRunStateCancelling,
		}).Updates(map[string]any{
		"state": ReactRunStateExpired,
	})
	if tx.Error != nil {
		return components.ErrorDbUpdate.Wrap(tx.Error)
	}
	return nil
}

func HasActiveReactRun(ctx *gin.Context, sessionID string) (bool, error) {
	return HasActiveReactRunWithDB(ctx, helpers.MysqlClientLLM, sessionID)
}

func HasActiveReactRunWithDB(ctx *gin.Context, db *gorm.DB, sessionID string) (bool, error) {
	var count int64
	err := db.Model(&ReactRun{}).WithContext(ctx).
		Where("session_id = ? AND state IN ?", sessionID, []string{
			ReactRunStateRunning,
			ReactRunStateWaitingClientMessage,
			ReactRunStateCancelling,
		}).Count(&count).Error
	if err != nil {
		return false, components.ErrorDbSelect.Wrap(err)
	}
	return count > 0, nil
}

func GetLatestReactRunBySessionIDWithDB(ctx *gin.Context, db *gorm.DB, sessionID string) (*ReactRun, error) {
	var run ReactRun
	err := db.Model(&ReactRun{}).WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at DESC, id DESC").First(&run).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &run, nil
}

func GetReactRunsBySessionID(ctx *gin.Context, sessionID string) ([]ReactRun, error) {
	return GetReactRunsBySessionIDWithDB(ctx, helpers.MysqlClientLLM, sessionID)
}

func GetReactRunsBySessionIDWithDB(ctx *gin.Context, db *gorm.DB, sessionID string) ([]ReactRun, error) {
	var runs []ReactRun
	err := db.Model(&ReactRun{}).WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at ASC, id ASC").Find(&runs).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return runs, nil
}

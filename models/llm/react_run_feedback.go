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
	ReactRunFeedbackDislike = -1
	ReactRunFeedbackNone    = 0
	ReactRunFeedbackLike    = 1
)

// ReactRunFeedback 存储某一轮(run)的当前点赞/点踩与问题反馈，按 run_id 唯一。
type ReactRunFeedback struct {
	ID                       uint       `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	RunID                    string     `json:"runId" gorm:"column:run_id;not null"`
	SessionID                string     `json:"sessionId" gorm:"column:session_id;not null"`
	UserName                 string     `json:"userName" gorm:"column:user_name;not null;default:''"`
	Feedback                 int        `json:"feedback" gorm:"column:feedback;not null;default:0"`
	ProblemFeedback          string     `json:"problemFeedback" gorm:"column:problem_feedback;not null;default:''"`
	FeedbackUpdatedAt        *time.Time `json:"feedbackUpdatedAt" gorm:"column:feedback_updated_at"`
	ProblemFeedbackUpdatedAt *time.Time `json:"problemFeedbackUpdatedAt" gorm:"column:problem_feedback_updated_at"`
	CreatedAt                time.Time  `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt                time.Time  `json:"updatedAt" gorm:"column:updated_at"`
}

func (f *ReactRunFeedback) TableName() string {
	return "tblLlmReactRunFeedback"
}

func GetReactRunFeedbackByRunID(ctx *gin.Context, runID string) (*ReactRunFeedback, error) {
	var fb ReactRunFeedback
	err := helpers.MysqlClientLLM.Model(&ReactRunFeedback{}).WithContext(ctx).
		Where("run_id = ?", runID).First(&fb).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &fb, nil
}

func GetReactRunFeedbacksBySessionID(ctx *gin.Context, sessionID string) ([]ReactRunFeedback, error) {
	var list []ReactRunFeedback
	err := helpers.MysqlClientLLM.Model(&ReactRunFeedback{}).WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("id ASC").Find(&list).Error
	if err != nil {
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return list, nil
}

// UpsertReactRunFeedback 按 run_id 幂等写入：只更新传入的字段，各自刷新对应时间戳。
func UpsertReactRunFeedback(ctx *gin.Context, runID, sessionID, userName string, feedback *int, problemFeedback *string) (*ReactRunFeedback, error) {
	now := time.Now()
	var result ReactRunFeedback
	err := helpers.MysqlClientLLM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing ReactRunFeedback
		findErr := tx.Where("run_id = ?", runID).First(&existing).Error
		isNew := errors.Is(findErr, gorm.ErrRecordNotFound)
		if findErr != nil && !isNew {
			return components.ErrorDbSelect.Wrap(findErr)
		}

		if isNew {
			// 只写本次触达的字段；未触达的时间戳交给列默认值（保持「未设置」），不写 now、不写 NULL。
			row := ReactRunFeedback{RunID: runID, SessionID: sessionID, UserName: userName}
			omit := make([]string, 0, 2)
			if feedback != nil {
				row.Feedback = *feedback
				row.FeedbackUpdatedAt = &now
			} else {
				omit = append(omit, "feedback_updated_at")
			}
			if problemFeedback != nil {
				row.ProblemFeedback = *problemFeedback
				row.ProblemFeedbackUpdatedAt = &now
			} else {
				omit = append(omit, "problem_feedback_updated_at")
			}
			db := tx
			if len(omit) > 0 {
				db = tx.Omit(omit...)
			}
			if err := db.Create(&row).Error; err != nil {
				return components.ErrorDbInsert.Wrap(err)
			}
		} else {
			// 只更新本次触达的列，未触达的点赞/问题时间保持原值。
			updates := make(map[string]any, 4)
			if feedback != nil {
				updates["feedback"] = *feedback
				updates["feedback_updated_at"] = now
			}
			if problemFeedback != nil {
				updates["problem_feedback"] = *problemFeedback
				updates["problem_feedback_updated_at"] = now
			}
			if len(updates) > 0 {
				if err := tx.Model(&ReactRunFeedback{}).Where("run_id = ?", runID).Updates(updates).Error; err != nil {
					return components.ErrorDbUpdate.Wrap(err)
				}
			}
		}

		if err := tx.Where("run_id = ?", runID).First(&result).Error; err != nil {
			return components.ErrorDbSelect.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

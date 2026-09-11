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
	ChatFileStatusParsed  = "parsed"
	ChatFileStatusDeleted = "deleted"
	ChatFileStatusExpired = "expired"
)

type ChatFileRecord struct {
	ID        uint      `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	FileID    string    `json:"fileId" gorm:"column:file_id;not null;uniqueIndex"`
	Owner     string    `json:"owner" gorm:"column:owner;not null"`
	FileName  string    `json:"fileName" gorm:"column:file_name;not null"`
	Ext       string    `json:"ext" gorm:"column:ext;not null"`
	MimeType  string    `json:"mimeType" gorm:"column:mime_type"`
	Charset   string    `json:"charset" gorm:"column:charset"`
	Size      int64     `json:"size" gorm:"column:size;not null"`
	CosKey    string    `json:"cosKey" gorm:"column:cos_key;not null"`
	CosURI    string    `json:"cosUri" gorm:"column:cos_uri"`
	Status    string    `json:"status" gorm:"column:status;not null"`
	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (m *ChatFileRecord) TableName() string {
	return "tblLlmChatFileRecord"
}

func CreateChatFileRecord(ctx *gin.Context, record *ChatFileRecord) error {
	err := helpers.MysqlClientLLM.Model(&ChatFileRecord{}).WithContext(ctx).Create(record).Error
	if err != nil {
		return components.ErrorDbInsert.Wrap(err)
	}
	return nil
}

func GetChatFileRecordByFileID(ctx *gin.Context, fileID string) (*ChatFileRecord, error) {
	return GetChatFileRecordByFileIDWithContext(ctx, fileID)
}

func GetChatFileRecordByFileIDWithContext(ctx context.Context, fileID string) (*ChatFileRecord, error) {
	var record ChatFileRecord
	err := helpers.MysqlClientLLM.Model(&ChatFileRecord{}).WithContext(ctx).
		Where("file_id = ?", fileID).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, components.ErrorDbSelect.Wrap(err)
	}
	return &record, nil
}

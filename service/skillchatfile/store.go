package skillchatfile

import (
	"encoding/json"
	"strings"

	"react-base-service/components"
	"react-base-service/helpers"

	"github.com/gin-gonic/gin"
)

// SaveMeta 将文件元数据写入 Redis
func SaveMeta(ctx *gin.Context, meta *ChatFileMeta) error {
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := helpers.RedisClient.Set(ctx, metaRedisKey(meta.FileID), string(b), MetaTTLSeconds); err != nil {
		return components.ErrorRedisSet.Sprintf(err.Error())
	}
	return nil
}

// GetMeta 读取文件元数据（供后续 chat 引用 fileId 时使用）
func GetMeta(ctx *gin.Context, fileID string) (*ChatFileMeta, error) {
	val, err := helpers.RedisClient.Get(ctx, metaRedisKey(fileID))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "nil") {
			return nil, nil
		}
		return nil, components.ErrorRedisGet.Sprintf(err.Error())
	}
	if len(val) == 0 {
		return nil, nil
	}
	var meta ChatFileMeta
	if err := json.Unmarshal([]byte(val), &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

package llm

import (
	"context"
	"encoding/json"
)

// marshalRequestBodyGuard 统一序列化 LLM 请求体。
func marshalRequestBodyGuard(_ context.Context, reqBody interface{}) ([]byte, error) {
	return json.Marshal(reqBody)
}

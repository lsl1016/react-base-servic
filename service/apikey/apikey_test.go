package apikey

import (
	"testing"

	model "react-base-service/models/llm"

	"github.com/stretchr/testify/require"
)

func TestToApiKeyRespOmitsValue(t *testing.T) {
	resp := ToApiKeyResp(&model.ApiKey{ID: 1, ApiKeyValue: "real-secret"})
	require.Empty(t, resp.ApiKey)
}

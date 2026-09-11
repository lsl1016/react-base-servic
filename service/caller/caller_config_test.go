package caller

import (
	"testing"

	model "react-base-service/models/llm"

	"github.com/stretchr/testify/require"
)

func TestCloneCallerConfigSnapshotRebuildsResourceRelations(t *testing.T) {
	snapshot := &model.CallerConfigSnapshot{
		Skills:           []model.Skill{{SkillID: "skill_old", CallerKey: "source", Name: "skill"}},
		SystemPrompts:    []model.SystemPrompt{{ID: 10, CallerKey: "source", Name: "prompt"}},
		Tools:            []model.Tool{{ToolID: "tool_old", CallerKey: "source", Name: "tool"}},
		ToolUserPolicies: []model.ToolUserPolicy{{ID: 11, ToolID: "tool_old", WhiteUserList: `["alice"]`, BlackUserList: `["bob"]`}},
		ApiKeys:          []model.ApiKey{{ID: 12, CallerKey: "source", Name: "key", ApiKeyValue: "secret"}},
	}

	cloneCallerConfigSnapshot(snapshot, "target", "operator")

	require.Equal(t, "target", snapshot.Skills[0].CallerKey)
	require.NotEqual(t, "skill_old", snapshot.Skills[0].SkillID)
	require.Equal(t, "target", snapshot.SystemPrompts[0].CallerKey)
	require.Zero(t, snapshot.SystemPrompts[0].ID)
	require.Equal(t, "target", snapshot.Tools[0].CallerKey)
	require.NotEqual(t, "tool_old", snapshot.Tools[0].ToolID)
	require.Equal(t, snapshot.Tools[0].ToolID, snapshot.ToolUserPolicies[0].ToolID)
	require.Equal(t, `["alice"]`, snapshot.ToolUserPolicies[0].WhiteUserList)
	require.Equal(t, `["bob"]`, snapshot.ToolUserPolicies[0].BlackUserList)
	require.Equal(t, "target", snapshot.ApiKeys[0].CallerKey)
	require.Equal(t, "secret", snapshot.ApiKeys[0].ApiKeyValue)
	require.Equal(t, "operator", snapshot.ToolUserPolicies[0].CreatedBy)
}

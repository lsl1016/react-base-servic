package tool

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	model "react-base-service/models/llm"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNormalizeToolUserList(t *testing.T) {
	got, err := normalizeToolUserList([]string{" user_b ", "", "user_a", "user_b"})
	require.NoError(t, err)
	require.Equal(t, []string{"user_a", "user_b"}, got)
}

func TestNormalizeToolUserListRejectsInvalidLists(t *testing.T) {
	_, err := normalizeToolUserList([]string{" ", ""})
	require.ErrorContains(t, err, "whiteUserList 不能为空")

	_, err = normalizeToolUserList([]string{strings.Repeat("用", maxToolUserNameLength+1)})
	require.ErrorContains(t, err, "用户名不能超过")

	tooManyUsers := make([]string, 0, maxToolUserPolicyUsers+1)
	for i := 0; i <= maxToolUserPolicyUsers; i++ {
		tooManyUsers = append(tooManyUsers, fmt.Sprintf("user_%03d", i))
	}
	_, err = normalizeToolUserList(tooManyUsers)
	require.ErrorContains(t, err, "whiteUserList 最多包含")
}

func TestNormalizeToolPolicyIDsDeduplicatesBeforeLimit(t *testing.T) {
	got, err := normalizeToolPolicyIDs([]string{" tool_b ", "tool_a", "tool_b"})
	require.NoError(t, err)
	require.Equal(t, []string{"tool_a", "tool_b"}, got)

	duplicates := make([]string, maxToolUserPolicyToolIDs+1)
	for i := range duplicates {
		duplicates[i] = "same_tool"
	}
	got, err = normalizeToolPolicyIDs(duplicates)
	require.NoError(t, err)
	require.Equal(t, []string{"same_tool"}, got)

	tooManyToolIDs := make([]string, 0, maxToolUserPolicyToolIDs+1)
	for i := 0; i <= maxToolUserPolicyToolIDs; i++ {
		tooManyToolIDs = append(tooManyToolIDs, fmt.Sprintf("tool_%03d", i))
	}
	_, err = normalizeToolPolicyIDs(tooManyToolIDs)
	require.ErrorContains(t, err, "toolIds 最多包含")
}

func TestFilterToolsByUserName(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	tools := []model.Tool{
		{ToolID: "unrestricted"},
		{ToolID: "allowed"},
		{ToolID: "denied"},
		{ToolID: "invalid"},
		{ToolID: "black-list-reserved"},
	}
	policies := []model.ToolUserPolicy{
		{ToolID: "allowed", WhiteUserList: `["user_a"]`},
		{ToolID: "denied", WhiteUserList: `["user_b"]`},
		{ToolID: "invalid", WhiteUserList: `[]`},
		{
			ToolID:        "black-list-reserved",
			WhiteUserList: `["user_a"]`,
			BlackUserList: `["user_a"]`,
		},
	}

	visible, filteredCount := filterToolsByUserName(ctx, tools, policies, "user_a")
	require.Equal(t, 2, filteredCount)
	require.Equal(t, []model.Tool{tools[0], tools[1], tools[4]}, visible)
}

func TestFilterToolsByUserNameUsesExactUserNameMatch(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	tools := []model.Tool{{ToolID: "restricted"}}
	policies := []model.ToolUserPolicy{{ToolID: "restricted", WhiteUserList: `["User_A"]`}}

	visible, filteredCount := filterToolsByUserName(ctx, tools, policies, "user_a")
	require.Empty(t, visible)
	require.Equal(t, 1, filteredCount)
}

func TestFilterToolsByUserNameFailsClosedForInvalidJSON(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	tools := []model.Tool{{ToolID: "restricted"}}
	policies := []model.ToolUserPolicy{{ToolID: "restricted", WhiteUserList: `{}`}}

	visible, filteredCount := filterToolsByUserName(ctx, tools, policies, "user_a")
	require.Empty(t, visible)
	require.Equal(t, 1, filteredCount)
}

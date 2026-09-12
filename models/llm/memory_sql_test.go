package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestMemoryOwnerScopeConditionCarriesStateInEveryBranch 防回归：
// owners OR 查询的每个分支必须带 state = active 条件（GORM Where().Or() 不会把先置
// 条件下推进 Or 分支，漏并会把软删条目泄漏进注入/列表）。
func TestMemoryOwnerScopeConditionCarriesStateInEveryBranch(t *testing.T) {
	owners := []MemoryOwner{
		BuildCallerMemoryOwner("demo-app"),
		BuildCallerUserMemoryOwner("demo-app", "zhangsan"),
	}
	condition, args := buildMemoryOwnerScopeCondition(owners)

	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "user:password@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	require.NoError(t, err)

	tx := db.Model(&MemoryItem{}).Where(condition, args...).Find(&MemoryItem{})
	require.NoError(t, tx.Error)

	sql := tx.Statement.SQL.String()
	require.Equal(t, 2, strings.Count(sql, "state = ?"), "每个 owner 分支都必须带 state 条件: %s", sql)
	require.Equal(t, 2, strings.Count(sql, "owner_type = ?"), "两个 owner 分支: %s", sql)
	require.Contains(t, sql, "(owner_type = ? AND owner_key = ? AND state = ?) OR (owner_type = ? AND owner_key = ? AND state = ?)", "分支必须括号包裹且逐支带 state: %s", sql)
}

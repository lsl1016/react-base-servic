package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openDryRunMysql(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "user:password@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	require.NoError(t, err)
	return db
}

// TestExpireStaleActiveReactRunsSQLShape 防回归：
// - WHERE 必须同时限定三种活跃 state 与 updated_at < cutoff（缺 updated_at 会在滚动重启时
//   误伤重启后仍在推进的 run；缺任一活跃 state 会留下永久阻塞会话的残留行）
// - UPDATE 必须把 state 置为 expired 并留下可观测的 error_message
func TestExpireStaleActiveReactRunsSQLShape(t *testing.T) {
	db := openDryRunMysql(t)

	cutoff := time.Date(2026, time.September, 13, 12, 0, 0, 0, time.Local)
	condition, args := staleActiveReactRunCondition(cutoff)
	tx := db.Model(&ReactRun{}).Where(condition, args...).
		Updates(map[string]any{
			"state":         ReactRunStateExpired,
			"error_message": "expired by stale active-run cleanup (service restart)",
		})
	require.NoError(t, tx.Error)

	sql := tx.Statement.SQL.String()
	require.Contains(t, sql, "state IN (?,?,?)", "三种活跃 state 都要进 IN 列表: %s", sql)
	require.Contains(t, sql, "updated_at < ?", "陈旧判定必须带 updated_at 下限: %s", sql)
	require.Contains(t, sql, "`state`=?", "必须更新 state: %s", sql)
	require.Contains(t, sql, "`error_message`=?", "必须留清理痕迹: %s", sql)

	vars := tx.Statement.Vars
	require.Len(t, vars, 7) // 3 state + cutoff + 新 state + error_message + gorm 自动维护的 updated_at
	stateCount := 0
	for _, want := range []string{
		ReactRunStateRunning,
		ReactRunStateWaitingClientMessage,
		ReactRunStateCancelling,
	} {
		for _, v := range vars {
			if s, ok := v.(string); ok && s == want {
				stateCount++
			}
		}
	}
	require.Equal(t, 3, stateCount, "三种活跃 state 各出现一次: %v", vars)
	require.Contains(t, vars, cutoff)
	require.Contains(t, vars, ReactRunStateExpired)
	require.Contains(t, vars, "expired by stale active-run cleanup (service restart)")
}

// TestStaleActiveReactRunConditionMatchesActiveSet 保证清理条件与 HasActiveReactRunWithDB 的
// 「活跃」口径一致：漏掉一种 state 就会出现清不掉的残留，多算则会误伤终态 run。
func TestStaleActiveReactRunConditionMatchesActiveSet(t *testing.T) {
	condition, args := staleActiveReactRunCondition(time.Now())
	require.Equal(t, "state IN ? AND updated_at < ?", condition)
	require.Len(t, args, 2)

	states, ok := args[0].([]string)
	require.True(t, ok, "首个参数必须是 state 列表: %v", args)
	require.ElementsMatch(t, []string{
		ReactRunStateRunning,
		ReactRunStateWaitingClientMessage,
		ReactRunStateCancelling,
	}, states)

	_, ok = args[1].(time.Time)
	require.True(t, ok, "第二个参数必须是 cutoff 时间: %v", args)
}

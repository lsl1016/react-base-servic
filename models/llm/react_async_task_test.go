package model

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestReactAsyncTaskAfterFindRestoresUnsetValues(t *testing.T) {
	progress := reactAsyncTaskUnsetProgress
	unsetTime := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	task := &ReactAsyncTask{
		Progress:       &progress,
		LastObservedAt: &unsetTime,
		CompletedAt:    &unsetTime,
		NextSyncAt:     &unsetTime,
		LeaseUntil:     &unsetTime,
	}

	require.NoError(t, task.AfterFind(nil))
	require.Nil(t, task.Progress)
	require.Nil(t, task.LastObservedAt)
	require.Nil(t, task.CompletedAt)
	require.Nil(t, task.NextSyncAt)
	require.Nil(t, task.LeaseUntil)
}

func TestReactAsyncTaskAfterFindKeepsObservedValues(t *testing.T) {
	progress := 0
	observedAt := time.Date(2026, time.September, 3, 10, 0, 0, 0, time.Local)
	task := &ReactAsyncTask{
		Progress:       &progress,
		LastObservedAt: &observedAt,
		CompletedAt:    &observedAt,
		NextSyncAt:     &observedAt,
		LeaseUntil:     &observedAt,
	}

	require.NoError(t, task.AfterFind(nil))
	require.Equal(t, 0, *task.Progress)
	require.Equal(t, observedAt, *task.LastObservedAt)
	require.Equal(t, observedAt, *task.CompletedAt)
	require.Equal(t, observedAt, *task.NextSyncAt)
	require.Equal(t, observedAt, *task.LeaseUntil)
}

func TestReactAsyncTaskDefaultFields(t *testing.T) {
	require.ElementsMatch(t,
		[]string{"Progress", "LastObservedAt", "CompletedAt", "NextSyncAt", "LeaseUntil"},
		reactAsyncTaskDefaultFields(&ReactAsyncTask{}),
	)

	now := time.Now()
	progress := 50
	task := &ReactAsyncTask{Progress: &progress, NextSyncAt: &now}
	require.ElementsMatch(t,
		[]string{"LastObservedAt", "CompletedAt", "LeaseUntil"},
		reactAsyncTaskDefaultFields(task),
	)
}

func TestReactAsyncTaskCreateOmitsUnsetFields(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "user:password@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	require.NoError(t, err)

	task := &ReactAsyncTask{
		SessionID:     "session-1",
		RunID:         "run-1",
		ToolUseID:     "tool-1",
		TaskInfo:      "{}",
		State:         ReactAsyncTaskStatePending,
		ExpireAt:      time.Now().Add(time.Hour),
		SchedulerType: "demo",
	}
	tx := db.Omit(reactAsyncTaskDefaultFields(task)...).Create(task)
	require.NoError(t, tx.Error)
	sql := strings.ToLower(tx.Statement.SQL.String())
	for _, column := range []string{"`progress`", "`last_observed_at`", "`completed_at`", "`next_sync_at`", "`lease_until`"} {
		require.NotContains(t, sql, column)
	}
}

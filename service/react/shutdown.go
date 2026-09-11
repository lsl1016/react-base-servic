package react

import (
	"context"
	"sort"

	model "react-base-service/models/llm"

	signalserver "react-base-service/golib/server/signal"
)

var registerReactShutdown = signalserver.RegisterShutdown
var expireReactRunsOnShutdown = model.ExpireReactRunsByRunIDs

func init() {
	registerReactRunShutdownHook()
}

func registerReactRunShutdownHook() {
	registerReactShutdown("reactRuns", func(ctx context.Context) error {
		runIDs := snapshotActiveReactRunIDs()
		if len(runIDs) == 0 {
			return nil
		}
		return expireReactRunsOnShutdown(ctx, runIDs)
	})
}

func snapshotActiveReactRunIDs() []string {
	runIDs := make([]string, 0)
	activeRunCancels.Range(func(key, _ any) bool {
		runID, ok := key.(string)
		if !ok || runID == "" {
			return true
		}
		runIDs = append(runIDs, runID)
		return true
	})
	sort.Strings(runIDs)
	return runIDs
}

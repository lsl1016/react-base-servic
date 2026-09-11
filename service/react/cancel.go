package react

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrReactRunCancelled       = errors.New("react run cancelled")
	ErrReactClientDisconnected = errors.New("react client disconnected")
	activeRunCancels           sync.Map // runID -> context.CancelCauseFunc
)

func registerReactRunCancel(runID string, cancel context.CancelCauseFunc) {
	activeRunCancels.Store(runID, cancel)
}

func unregisterReactRunCancel(runID string) {
	activeRunCancels.Delete(runID)
}

func getActiveReactRunCancel(runID string) (context.CancelCauseFunc, bool) {
	value, ok := activeRunCancels.Load(runID)
	if !ok {
		return nil, false
	}
	cancel, ok := value.(context.CancelCauseFunc)
	return cancel, ok
}

func IsReactRunCancelled(err error) bool {
	return errors.Is(err, ErrReactRunCancelled) || errors.Is(err, context.Canceled)
}

func IsReactClientDisconnected(err error) bool {
	return errors.Is(err, ErrReactClientDisconnected)
}

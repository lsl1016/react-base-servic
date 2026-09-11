package react

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

func TestRegisterReactRunShutdownHookExpiresActiveRuns(t *testing.T) {
	originalRegister := registerReactShutdown
	originalExpire := expireReactRunsOnShutdown
	defer func() {
		registerReactShutdown = originalRegister
		expireReactRunsOnShutdown = originalExpire
	}()

	var registeredName string
	var registeredShutdown func(context.Context) error
	registerReactShutdown = func(name string, shutdown func(context.Context) error) {
		registeredName = name
		registeredShutdown = shutdown
	}

	var expiredRunIDs []string
	expireReactRunsOnShutdown = func(_ context.Context, runIDs []string) error {
		expiredRunIDs = append([]string(nil), runIDs...)
		return nil
	}

	registerReactRunShutdownHook()
	if registeredName != "reactRuns" {
		t.Fatalf("expected shutdown service name reactRuns, got %q", registeredName)
	}
	if registeredShutdown == nil {
		t.Fatal("expected shutdown callback to be registered")
	}

	registerReactRunCancel("run-b", func(error) {})
	registerReactRunCancel("run-a", func(error) {})
	defer unregisterReactRunCancel("run-b")
	defer unregisterReactRunCancel("run-a")

	if err := registeredShutdown(context.Background()); err != nil {
		t.Fatalf("expected shutdown callback to succeed, got %v", err)
	}

	sort.Strings(expiredRunIDs)
	if !reflect.DeepEqual(expiredRunIDs, []string{"run-a", "run-b"}) {
		t.Fatalf("expected active runs to expire, got %v", expiredRunIDs)
	}
}

func TestRegisterReactRunShutdownHookSkipsWhenNoActiveRuns(t *testing.T) {
	originalRegister := registerReactShutdown
	originalExpire := expireReactRunsOnShutdown
	defer func() {
		registerReactShutdown = originalRegister
		expireReactRunsOnShutdown = originalExpire
	}()

	var registeredShutdown func(context.Context) error
	registerReactShutdown = func(_ string, shutdown func(context.Context) error) {
		registeredShutdown = shutdown
	}

	called := false
	expireReactRunsOnShutdown = func(_ context.Context, runIDs []string) error {
		called = true
		if len(runIDs) != 0 {
			t.Fatalf("expected no run IDs, got %v", runIDs)
		}
		return nil
	}

	registerReactRunShutdownHook()
	if registeredShutdown == nil {
		t.Fatal("expected shutdown callback to be registered")
	}

	if err := registeredShutdown(context.Background()); err != nil {
		t.Fatalf("expected shutdown callback to succeed, got %v", err)
	}
	if called {
		t.Fatal("expected no expiration call when no active runs exist")
	}
}

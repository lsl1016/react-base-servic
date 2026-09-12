package react

import (
	"strings"
	"testing"

	model "react-base-service/models/llm"
)

func TestValidateMemoryOwnerCallerScope(t *testing.T) {
	owner, err := validateMemoryOwner("caller", "demo-app")
	if err != nil {
		t.Fatalf("caller owner should pass: %v", err)
	}
	if owner != model.BuildCallerMemoryOwner("demo-app") {
		t.Fatalf("unexpected owner: %+v", owner)
	}
}

func TestValidateMemoryOwnerCallerUserScope(t *testing.T) {
	owner, err := validateMemoryOwner("caller_user", "demo-app|zhangsan")
	if err != nil {
		t.Fatalf("caller_user owner should pass: %v", err)
	}
	if owner.OwnerKey != "demo-app|zhangsan" || owner.OwnerType != model.MemoryOwnerTypeCallerUser {
		t.Fatalf("unexpected owner: %+v", owner)
	}
}

func TestValidateMemoryOwnerRejectsInvalid(t *testing.T) {
	if _, err := validateMemoryOwner("route", "x"); err == nil || !strings.Contains(err.Error(), "ownerType") {
		t.Fatalf("unknown ownerType should fail, got %v", err)
	}
	if _, err := validateMemoryOwner("caller", "  "); err == nil || !strings.Contains(err.Error(), "ownerKey") {
		t.Fatalf("empty caller ownerKey should fail, got %v", err)
	}
	if _, err := validateMemoryOwner("caller_user", "no-separator"); err == nil || !strings.Contains(err.Error(), "格式") {
		t.Fatalf("caller_user ownerKey without separator should fail, got %v", err)
	}
}

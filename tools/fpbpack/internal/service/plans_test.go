package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidPlanID(t *testing.T) {
	if !validPlanID("plan-0123456789abcdef") {
		t.Fatal("valid plan id rejected")
	}
	for _, value := range []string{"../plan-x", "plan-short", "other-0123456789abcdef"} {
		if validPlanID(value) {
			t.Fatalf("invalid plan id accepted: %q", value)
		}
	}
}

func TestPlansAndHistoryReturnEmptyForFreshState(t *testing.T) {
	service := &Service{options: Options{StateDir: t.TempDir()}}
	plans, err := service.Plans()
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 0 {
		t.Fatalf("plans = %d, want 0", len(plans))
	}
	history, err := service.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("history = %d, want 0", len(history))
	}
	if _, err := os.Stat(filepath.Join(service.options.StateDir, "plans")); !os.IsNotExist(err) {
		t.Fatalf("reading plans should not create directories")
	}
}

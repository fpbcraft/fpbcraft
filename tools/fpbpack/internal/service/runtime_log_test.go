package service

import (
	"errors"
	"fmt"
	"testing"
)

func TestRuntimeLogsAreBoundedNewestFirst(t *testing.T) {
	s := &Service{}
	for index := 0; index < runtimeLogLimit+5; index++ {
		s.logEvent("info", "test", fmt.Sprintf("entry-%03d", index))
	}
	entries := s.Logs(runtimeLogLimit)
	if len(entries) != runtimeLogLimit {
		t.Fatalf("entries = %d, want %d", len(entries), runtimeLogLimit)
	}
	if entries[0].Message != "entry-504" {
		t.Fatalf("newest entry = %q", entries[0].Message)
	}
	if entries[len(entries)-1].Message != "entry-005" {
		t.Fatalf("oldest retained entry = %q", entries[len(entries)-1].Message)
	}
}

func TestModManagementErrorsAreLogged(t *testing.T) {
	s := &Service{}
	s.logModManagement("adopt_current", ModManagementResult{}, errors.New("verification failed"))
	entries := s.Logs(10)
	if len(entries) != 1 || entries[0].Level != "error" || entries[0].Area != "mods" {
		t.Fatalf("unexpected log entry: %+v", entries)
	}
}

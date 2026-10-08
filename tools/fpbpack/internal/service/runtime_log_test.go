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
	if entries[0].Message != fmt.Sprintf("entry-%03d", runtimeLogLimit+4) {
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

func TestRuntimeCommandWriterStreamsCompleteLines(t *testing.T) {
	var lines []string
	writer := &runtimeCommandWriter{onLine: func(line string) { lines = append(lines, line) }}
	if _, err := writer.Write([]byte("Installing\nResolving dep")); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "Installing" {
		t.Fatalf("expected only the first complete line, got %v", lines)
	}
	if _, err := writer.Write([]byte("endencies\nFinished")); err != nil {
		t.Fatal(err)
	}
	writer.Flush()
	want := []string{"Installing", "Resolving dependencies", "Finished"}
	if len(lines) != len(want) {
		t.Fatalf("lines = %v", lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("lines[%d] = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestRefreshProgressAddsIndividualLogEntries(t *testing.T) {
	s := &Service{}
	s.beginRefresh("updates", "Checking updates")
	s.setRefreshProgress("providers", "Checked Create", 1, 2, 50)
	s.setRefreshProgress("providers", "Checked Create", 1, 2, 50)
	s.setRefreshProgress("providers", "Checked Fabric API", 2, 2, 100)
	entries := s.Logs(10)
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want start and two progress steps", len(entries))
	}
	if entries[0].Message != "Checked Fabric API (2/2)" ||
		entries[1].Message != "Checked Create (1/2)" {
		t.Fatalf("unexpected progress entries: %+v", entries)
	}
}

package service

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureArtifactDownloadsAndVerifiesSHA512(t *testing.T) {
	content := []byte("verified artifact")
	sum := sha512.Sum512(content)
	expected := hex.EncodeToString(sum[:])
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))

	target := filepath.Join(t.TempDir(), "cache", expected+".jar")
	size, err := ensureArtifact(context.Background(), server.URL, expected, target)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	if size != int64(len(content)) {
		server.Close()
		t.Fatalf("size = %d", size)
	}
	bytes, err := os.ReadFile(target)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	if string(bytes) != string(content) {
		server.Close()
		t.Fatalf("unexpected cached content %q", string(bytes))
	}

	server.Close()
	if _, err := ensureArtifact(context.Background(), server.URL, expected, target); err != nil {
		t.Fatalf("verified cache was not reused: %v", err)
	}
}

func TestEnsureArtifactRejectsHashMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("wrong"))
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "artifact.jar")
	if _, err := ensureArtifact(context.Background(), server.URL, strings.Repeat("0", 128), target); err == nil {
		t.Fatal("expected hash mismatch")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("mismatched artifact should not be persisted")
	}
}

package service

import (
	"context"
	"crypto/sha1"
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
	verified, err := ensureArtifact(context.Background(), server.URL, expected, "", "", target)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	if verified.Bytes != int64(len(content)) {
		server.Close()
		t.Fatalf("size = %d", verified.Bytes)
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
	if _, err := ensureArtifact(context.Background(), server.URL, expected, "", "", target); err != nil {
		t.Fatalf("verified cache was not reused: %v", err)
	}
}

func TestEnsureArtifactRejectsHashMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("wrong"))
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "artifact.jar")
	if _, err := ensureArtifact(context.Background(), server.URL, strings.Repeat("0", 128), "", "", target); err == nil {
		t.Fatal("expected hash mismatch")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("mismatched artifact should not be persisted")
	}
}

func TestEnsureArtifactVerifiesSHA1AndComputesSHA512(t *testing.T) {
	content := []byte("curseforge-style artifact")
	sha1Sum := sha1.Sum(content)
	expectedSHA1 := hex.EncodeToString(sha1Sum[:])
	sha512Sum := sha512.Sum512(content)
	expectedSHA512 := hex.EncodeToString(sha512Sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "artifact.jar")
	verified, err := ensureArtifact(context.Background(), server.URL, "", "", expectedSHA1, target)
	if err != nil {
		t.Fatal(err)
	}
	if verified.SHA1 != expectedSHA1 {
		t.Fatalf("sha1 = %s", verified.SHA1)
	}
	if verified.SHA512 != expectedSHA512 {
		t.Fatalf("sha512 = %s", verified.SHA512)
	}
}

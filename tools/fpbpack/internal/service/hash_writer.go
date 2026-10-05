package service

import (
	"crypto/sha512"
	"encoding/hex"
	"hash"
)

type sha512Writer struct {
	hash hash.Hash
}

func newSHA512Writer() *sha512Writer {
	return &sha512Writer{hash: sha512.New()}
}

func (w *sha512Writer) Write(p []byte) (int, error) {
	return w.hash.Write(p)
}

func (w *sha512Writer) Sum() string {
	return hex.EncodeToString(w.hash.Sum(nil))
}

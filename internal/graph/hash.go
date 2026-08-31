package graph

import (
	"crypto/sha256"
	"encoding/hex"
)

// ContentHash returns a stable SHA-256 hex digest over a node's
// content-defining text (e.g. signature + body for a function, the field
// list for a struct). Callers are responsible for normalizing the input
// (e.g. stripping comments/whitespace) before hashing, so that
// content-irrelevant formatting changes don't produce a new hash.
func ContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

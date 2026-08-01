package identity

import "testing"

func TestNewTokenProducesVerifiableDigest(t *testing.T) {
	token, digest, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken() error = %v", err)
	}
	if token == "" || len(digest) == 0 {
		t.Fatal("NewToken() returned an empty token or digest")
	}
	if !VerifyToken(digest, token) {
		t.Fatal("VerifyToken() = false, want true")
	}
	if VerifyToken(digest, token+"x") {
		t.Fatal("VerifyToken() = true for a different token")
	}
}

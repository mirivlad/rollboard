package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
)

func NewToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, DigestToken(token), nil
}

func DigestToken(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return digest[:]
}

func VerifyToken(digest []byte, token string) bool {
	return subtle.ConstantTimeCompare(digest, DigestToken(token)) == 1
}

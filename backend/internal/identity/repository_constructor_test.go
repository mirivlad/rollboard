package identity

import "testing"

func TestNewRepositoryRejectsNilPool(t *testing.T) {
	if _, err := NewRepository(nil); err == nil {
		t.Fatal("NewRepository(nil) error = nil, want error")
	}
}

package identity

import "testing"

func TestUserPublicProfileNeverContainsPasswordHash(t *testing.T) {
	profile := User{ID: "user-id", Email: "author@example.com", DisplayName: "Author"}.Public()
	if profile.Email != "author@example.com" || profile.DisplayName != "Author" {
		t.Fatalf("Public() = %#v", profile)
	}
}

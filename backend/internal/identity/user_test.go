package identity

import "testing"

func TestRegistrationInputValidate(t *testing.T) {
	valid := RegistrationInput{Email: "Author@Example.com", DisplayName: "Author", Password: "long-enough-password"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if valid.Email != "author@example.com" {
		t.Fatalf("Email = %q, want normalized lowercase", valid.Email)
	}

	for _, input := range []RegistrationInput{
		{Email: "bad", DisplayName: "Author", Password: "long-enough-password"},
		{Email: "a@example.com", DisplayName: "", Password: "long-enough-password"},
		{Email: "a@example.com", DisplayName: "Author", Password: "short"},
	} {
		if err := input.Validate(); err == nil {
			t.Fatalf("Validate() error = nil for %#v", input)
		}
	}
}

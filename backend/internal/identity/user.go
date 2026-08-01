package identity

import (
	"fmt"
	"net/mail"
	"strings"
)

type RegistrationInput struct {
	Email       string
	DisplayName string
	Password    string
}

func (i *RegistrationInput) Validate() error {
	i.Email = strings.ToLower(strings.TrimSpace(i.Email))
	i.DisplayName = strings.TrimSpace(i.DisplayName)
	if _, err := mail.ParseAddress(i.Email); err != nil || !strings.Contains(i.Email, "@") {
		return fmt.Errorf("email must be valid")
	}
	if i.DisplayName == "" || len([]rune(i.DisplayName)) > 64 {
		return fmt.Errorf("display name must contain 1 to 64 characters")
	}
	if len(i.Password) < 12 || len(i.Password) > 1024 {
		return fmt.Errorf("password must contain 12 to 1024 characters")
	}
	return nil
}

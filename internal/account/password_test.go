package account

import (
	"strings"
	"testing"
)

func TestPasswords(t *testing.T) {
	password := "a long password with spaces"
	first, second := HashPassword(password), HashPassword(password)
	if first == second {
		t.Fatal("password salts must differ")
	}
	if !VerifyPassword(first, password) || VerifyPassword(first, "wrong password") {
		t.Fatal("incorrect password verification")
	}
	for _, malformed := range []string{"", "plain text", strings.Replace(first, "m=19456", "m=999999999", 1), hashPrefix + "bad$bad"} {
		if VerifyPassword(malformed, password) {
			t.Fatal("malformed hash accepted")
		}
	}
}

func TestCredentialValidation(t *testing.T) {
	for _, password := range []string{"short", strings.Repeat("a", 129), string([]byte{0xff})} {
		if validPassword(password) {
			t.Fatal("invalid password accepted")
		}
	}
	if !validPassword(strings.Repeat("я", 15)) {
		t.Fatal("length must count Unicode characters")
	}
	if normalize(" Player_1 ") != "player_1" {
		t.Fatal("username normalization failed")
	}
	for _, token := range []string{"", "abc", strings.Repeat("a", 5000)} {
		if _, err := tokenHash(token); err == nil {
			t.Fatal("invalid session token accepted")
		}
	}
}

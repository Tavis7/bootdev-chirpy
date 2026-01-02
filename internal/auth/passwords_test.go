package auth

import (
	"testing"
)

var p = "pa$$word"

func TestHashPassword(t *testing.T) {
	h, err := HashPassword(p)
	if err != nil {
		t.Errorf("Got error: %v", err)
	}
	if len(h) <= 0 {
		t.Errorf("empty hash: %v", h)
	}
}

func TestCheckPasswordHash(t *testing.T) {
	h := "$argon2id$v=19$m=800000,t=1,p=1$/bxE+IPZO0qLotrvoNoS2g$wQb9oQQXCveFl2gDHqfi6A"

	matches, err := CheckPasswordHash("pa$$word", h)
	if err != nil {
		t.Errorf("Got error: %v", err)
		return
	}

	if !matches {
		t.Errorf("Password hash doesn't match: '%v'", h)
	}

	matches, err = CheckPasswordHash("p4$$word", h)
	if err != nil {
		t.Errorf("Got error: %v", err)
		return
	}

	if matches {
		t.Errorf("Password hash matches: '%v'", h)
	}
}

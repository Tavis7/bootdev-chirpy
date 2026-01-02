package auth

import (
	"testing"
	"time"
	"github.com/google/uuid"
)

func TestMakeJWT(t *testing.T) {
	// uuid.New()
	id, err := uuid.Parse("253be0c3-c9e8-4d34-b6a9-9a8211884bc3")
	if err != nil {
		t.Errorf("Test is broken: %v", err)
	}
	s, err := MakeJWT(id, "abcdef", time.Second * 60)
	if err != nil {
		t.Errorf("%v, %v", s, err)
		return
	}
}

func TestMakeClaims(t *testing.T) {
	// uuid.New()
	id, err := uuid.Parse("253be0c3-c9e8-4d34-b6a9-9a8211884bc3")
	if err != nil {
		t.Errorf("Test is broken: %v", err)
	}
	key := "abcdef"
	s, err := MakeJWT(id, key, time.Second * 60)
	if err != nil {
		t.Errorf("%v, %v", s, err)
	}

	validatedId, err := ValidateJWT(s, key)
	if err != nil {
		t.Errorf("Error validating: %v", err)
		t.Errorf("s: %v", s)
		return
	}
	if validatedId != id {
		t.Errorf("Validation reurned wrong ID: %v != %v", id, validatedId)
	}


	validatedId, err = ValidateJWT(s, "wasd")
	if err == nil {
		t.Errorf("Validation should error: %v", err)
	}
	if validatedId == id {
		t.Errorf("Validation should error, but: %v == %v", id, validatedId)
	}
}

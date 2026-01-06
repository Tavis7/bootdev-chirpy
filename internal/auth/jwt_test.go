package auth

import (
	"github.com/google/uuid"
	"net/http"
	"testing"
	"time"
)

var key = "XJaqElLgyOn5Qz1bkyZyDGGPdooJkYnAaw/KAfdoc8txDLwzZxCmobL4Iwsvdp00eIWmYTff9MgGjvxe7E1/Ng=="

func TestMakeJWT(t *testing.T) {
	// uuid.New()
	id, err := uuid.Parse("253be0c3-c9e8-4d34-b6a9-9a8211884bc3")
	if err != nil {
		t.Errorf("Test is broken: %v", err)
	}
	s, err := MakeJWT(id, key, time.Second*60)
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
	s, err := MakeJWT(id, key, time.Second*60)
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

func TestGetBearerToken(t *testing.T) {
	header := http.Header{}
	header.Set("Authorization", "Bearer abc")
	token, err := GetBearerToken(header)
	if err != nil {
		t.Errorf("Token expected but got error: %v", err)
		return
	}

	if token != "abc" {
		t.Errorf("Expected 'abc' but got '%v'", token)
	}

	header = http.Header{}
	header.Set("Authorization", "Bearer    abc     ")
	token, err = GetBearerToken(header)
	if err != nil {
		t.Errorf("Token expected but got error: %v", err)
		return
	}

	if token != "abc" {
		t.Errorf("Expected 'abc' but got '%v'", token)
	}
}

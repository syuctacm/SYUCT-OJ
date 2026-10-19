package user

import "testing"

func TestHydroPasswordHash(t *testing.T) {
	hash := HashPassword("secret", "salt")
	if len(hash) != 64 {
		t.Fatalf("hash length = %d, want 64", len(hash))
	}
	u := &User{Salt: "salt", Hash: hash, HashType: "hydro"}
	if err := VerifyPassword(u, "secret"); err != nil {
		t.Fatalf("valid password rejected: %v", err)
	}
	if err := VerifyPassword(u, "wrong"); err == nil {
		t.Fatal("wrong password accepted")
	}
	if err := VerifyPassword(&User{HashType: "unsupported"}, "secret"); err == nil {
		t.Fatal("unsupported hash type accepted")
	}
}

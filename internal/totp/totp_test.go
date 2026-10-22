package totp

import (
	"strings"
	"testing"
	"time"
)

func TestCodeAndVerify(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) != SecretLength {
		t.Fatalf("secret length = %d", len(secret))
	}
	now := time.Unix(1_700_000_000, 0)
	code, err := Code(secret, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != Digits {
		t.Fatalf("code = %q", code)
	}
	if !Verify(secret, code, now) {
		t.Fatal("current code should verify")
	}
	if !Verify(strings.ToLower(secret), code, now) {
		t.Fatal("lowercase secret should verify")
	}
	if !Verify(secret, code, now.Add(Period*time.Second)) {
		t.Fatal("previous time step should verify")
	}
	if !Verify(secret, code, now.Add(-Period*time.Second)) {
		t.Fatal("next time step should verify")
	}
	if Verify(secret, code, now.Add(2*Period*time.Second)) {
		t.Fatal("code outside the allowed window should not verify")
	}
	if Verify(secret, "123", now) || Verify(secret, "abcdef", now) {
		t.Fatal("malformed codes should not verify")
	}
}

func TestKnownTOTPValue(t *testing.T) {
	// RFC 6238 SHA-1 vector truncated to six digits: at T=59, the
	// eight-digit value is 94287082.
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	code, err := Code(secret, time.Unix(59, 0))
	if err != nil {
		t.Fatal(err)
	}
	if code != "287082" {
		t.Fatalf("code = %q, want 287082", code)
	}
}

func TestProvisioningURI(t *testing.T) {
	uri := ProvisioningURI("SYUCT OJ", "alice", "ABCDEFGHIJKLMN")
	if !strings.HasPrefix(uri, "otpauth://totp/") || !strings.Contains(uri, "secret=ABCDEFGHIJKLMN") || !strings.Contains(uri, "issuer=SYUCT+OJ") {
		t.Fatalf("uri = %q", uri)
	}
}

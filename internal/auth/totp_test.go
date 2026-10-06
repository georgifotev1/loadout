package auth

import (
	"bytes"
	"net/url"
	"regexp"
	"testing"
	"time"
)

// RFC 6238 appendix B, SHA-1 rows; the RFC's codes have 8 digits, ours are their last 6.
func TestTOTPCodeRFCVectors(t *testing.T) {
	secret := []byte("12345678901234567890")
	want := map[int64]string{
		59:          "287082",
		1111111109:  "081804",
		1111111111:  "050471",
		1234567890:  "005924",
		2000000000:  "279037",
		20000000000: "353130",
	}
	for unix, code := range want {
		if got := totpCode(secret, totpStep(time.Unix(unix, 0))); got != code {
			t.Errorf("t=%d: code %s, want %s", unix, got, code)
		}
	}
}

func TestMatchTOTP(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1111111111, 0)
	cur := totpStep(now)
	code := func(step int64) string { return totpCode(secret, step) }

	tests := []struct {
		name     string
		code     string
		after    int64
		wantStep int64
		ok       bool
	}{
		{"current", code(cur), 0, cur, true},
		{"one step behind", code(cur - 1), 0, cur - 1, true},
		{"one step ahead", code(cur + 1), 0, cur + 1, true},
		{"two steps behind", code(cur - 2), 0, 0, false},
		{"already used", code(cur), cur, 0, false},
		{"older than the last used", code(cur - 1), cur, 0, false},
		{"newer than the last used", code(cur + 1), cur, cur + 1, true},
		{"too short", code(cur)[:5], 0, 0, false},
		{"empty", "", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, ok := matchTOTP(secret, tt.code, now, tt.after)
			if ok != tt.ok || step != tt.wantStep {
				t.Errorf("step %d ok %v, want %d %v", step, ok, tt.wantStep, tt.ok)
			}
		})
	}
}

func TestTOTPURI(t *testing.T) {
	u, err := url.Parse(totpURI([]byte("12345678901234567890"), "admin+1@loadout.bg"))
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "otpauth" || u.Host != "totp" || u.Path != "/Loadout:admin+1@loadout.bg" {
		t.Errorf("uri %s", u)
	}
	q := u.Query()
	if q.Get("secret") != "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" || q.Get("issuer") != "Loadout" ||
		q.Get("digits") != "6" || q.Get("period") != "30" || q.Get("algorithm") != "SHA1" {
		t.Errorf("query %v", q)
	}
}

func TestSealer(t *testing.T) {
	s, err := newSealer(bytes.Repeat([]byte{7}, TOTPKeyLen))
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte("a totp secret")
	sealed := s.seal(secret, []byte("user-1"))
	if bytes.Contains(sealed, secret) {
		t.Fatal("the secret is in the ciphertext")
	}
	if got, err := s.open(sealed, []byte("user-1")); err != nil || !bytes.Equal(got, secret) {
		t.Errorf("open: %q, %v", got, err)
	}
	if _, err := s.open(sealed, []byte("user-2")); err != errBadCiphertext {
		t.Errorf("another user's row: err %v", err)
	}
	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 1
	if _, err := s.open(tampered, []byte("user-1")); err != errBadCiphertext {
		t.Errorf("tampered: err %v", err)
	}
	other, _ := newSealer(bytes.Repeat([]byte{8}, TOTPKeyLen))
	if _, err := other.open(sealed, []byte("user-1")); err != errBadCiphertext {
		t.Errorf("other key: err %v", err)
	}
	if _, err := newSealer([]byte("short")); err == nil {
		t.Error("a short key was accepted")
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes := newRecoveryCodes()
	if len(codes) != recoveryCodeCount {
		t.Fatalf("%d codes", len(codes))
	}
	format := regexp.MustCompile(`^[a-z2-7]{4}-[a-z2-7]{4}-[a-z2-7]{4}-[a-z2-7]{4}$`)
	seen := map[string]bool{}
	for _, c := range codes {
		if !format.MatchString(c) || seen[c] {
			t.Errorf("code %q", c)
		}
		seen[c] = true
	}

	c := codes[0]
	plain := c[0:4] + c[5:9] + c[10:14] + c[15:19]
	for _, typed := range []string{c, plain, " " + c + " ", "  " + c[0:4] + " " + c[5:9] + " " + c[10:14] + " " + c[15:19]} {
		if got := normalizeRecoveryCode(typed); got != plain {
			t.Errorf("%q: normalized %q, want %q", typed, got, plain)
		}
	}
	upper := normalizeRecoveryCode("ABCD-EFGH-2345-67AB")
	if upper != "abcdefgh234567ab" {
		t.Errorf("upper case: %q", upper)
	}
	for _, bad := range []string{"", "123456", "abcd-efgh-2345-67a", "abcd-efgh-2345-67a1"} {
		if got := normalizeRecoveryCode(bad); got != "" {
			t.Errorf("%q: normalized %q, want none", bad, got)
		}
	}
}

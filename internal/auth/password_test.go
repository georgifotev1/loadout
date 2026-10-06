package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/gfotev/loadout/internal/platform/validate"
)

// cheap keeps hashing fast in tests; the format and the checks don't depend on the cost.
var cheap = params{time: 1, memory: 8 * 1024, threads: 1}

func TestHashPassword(t *testing.T) {
	h := hashPassword("correct horse battery", cheap)
	if !strings.HasPrefix(h, "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Fatalf("hash %q", h)
	}
	if h == hashPassword("correct horse battery", cheap) {
		t.Error("two hashes of one password are equal: the salt isn't random")
	}

	ok, p, err := verifyPassword("correct horse battery", h)
	if err != nil || !ok || p != cheap {
		t.Errorf("right password: ok=%v params=%+v err=%v", ok, p, err)
	}
	ok, _, err = verifyPassword("correct horse battery!", h)
	if err != nil || ok {
		t.Errorf("wrong password: ok=%v err=%v", ok, err)
	}
}

func TestVerifyPasswordMalformed(t *testing.T) {
	good := hashPassword("correct horse battery", cheap)
	parts := strings.Split(good, "$")
	for name, h := range map[string]string{
		"empty":       "",
		"bcrypt":      "$2a$10$abcdefghijklmnopqrstuuJ1bNYmQ3QeUGqd8xZpM1v6xgYzG5Bz2",
		"argon2i":     strings.Replace(good, "argon2id", "argon2i", 1),
		"old version": strings.Replace(good, "v=19", "v=16", 1),
		"zero time":   strings.Replace(good, "t=1", "t=0", 1),
		"bad salt":    strings.Join([]string{"", parts[1], parts[2], parts[3], "!!", parts[5]}, "$"),
		"no key":      strings.Join([]string{"", parts[1], parts[2], parts[3], parts[4], ""}, "$"),
	} {
		if _, _, err := verifyPassword("correct horse battery", h); err != errMalformedHash {
			t.Errorf("%s: err %v, want errMalformedHash", name, err)
		}
	}
}

func TestCheckPassword(t *testing.T) {
	tests := []struct {
		pw   string
		code string
	}{
		{"123456789", "too_short"},
		{"1234567890", ""},
		{"пароласдес", ""}, // 10 letters, 20 bytes: length is in characters
		{strings.Repeat("a", MaxPasswordLen), ""},
		{strings.Repeat("a", MaxPasswordLen+1), "too_long"},
	}
	for _, tt := range tests {
		var v validate.Validator
		checkPassword(&v, "password", tt.pw)
		var code string
		if err, ok := v.Err().(*validate.Error); ok {
			code = err.Fields[0].Code
		}
		if code != tt.code {
			t.Errorf("%q: code %q, want %q", tt.pw, code, tt.code)
		}
	}
}

func TestLockFor(t *testing.T) {
	l := limit{free: 5, firstLock: 30 * time.Second, maxLock: 5 * time.Minute}
	want := map[int]time.Duration{
		1:  0,
		5:  0,
		6:  30 * time.Second,
		7:  time.Minute,
		8:  2 * time.Minute,
		9:  4 * time.Minute,
		10: 5 * time.Minute,
		50: 5 * time.Minute,
	}
	for failures, d := range want {
		if got := l.lockFor(failures); got != d {
			t.Errorf("failure %d: lock %s, want %s", failures, got, d)
		}
	}
}

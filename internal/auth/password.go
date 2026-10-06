package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"github.com/gfotev/loadout/internal/platform/validate"
)

// Password length limits. No composition rules: length is what makes a password hard to guess.
const (
	MinPasswordLen = 10
	MaxPasswordLen = 128
)

// params are argon2id's cost parameters. They're stored in every hash, so raising them only
// changes new hashes, and Login rehashes old ones.
type params struct {
	time    uint32
	memory  uint32 // KiB
	threads uint8
}

// defaultParams is RFC 9106's second recommended option: 64 MiB, 3 passes, 4 lanes. About 50 ms
// on a laptop.
var defaultParams = params{time: 3, memory: 64 * 1024, threads: 4}

const (
	saltLen = 16
	keyLen  = 32
)

var errMalformedHash = errors.New("auth: malformed password hash")

// checkPassword records a field error if pw is too short or too long.
func checkPassword(v *validate.Validator, field, pw string) {
	n := utf8.RuneCountInString(pw)
	v.Check(n >= MinPasswordLen, field, "too_short")
	v.Check(n <= MaxPasswordLen, field, "too_long")
}

// hashPassword returns pw's argon2id hash in the PHC string format:
// $argon2id$v=19$m=65536,t=3,p=4$<salt>$<key>
func hashPassword(pw string, p params) string {
	salt := make([]byte, saltLen)
	_, _ = rand.Read(salt) // crypto/rand.Read never returns an error
	key := argon2.IDKey([]byte(pw), salt, p.time, p.memory, p.threads, keyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.time, p.threads, b64.EncodeToString(salt), b64.EncodeToString(key))
}

// verifyPassword reports whether pw matches encoded, and the parameters encoded was made with.
func verifyPassword(pw, encoded string) (bool, params, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, params{}, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, params{}, errMalformedHash
	}
	var p params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil ||
		p.memory == 0 || p.time == 0 || p.threads == 0 {
		return false, params{}, errMalformedHash
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, params{}, errMalformedHash
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, params{}, errMalformedHash
	}

	got := argon2.IDKey([]byte(pw), salt, p.time, p.memory, p.threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, p, nil
}

package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP as in RFC 6238 with the parameters every authenticator app supports: HMAC-SHA1, 6 digits,
// 30-second steps.
const (
	totpPeriod    = 30
	totpDigits    = 6
	totpSecretLen = 20 // 160 bits, RFC 4226's recommendation for SHA-1
	// totpSkew is how many steps a code may be off, either way, for a phone clock that drifts.
	totpSkew = 1
)

// TOTPIssuer is the account label's prefix in the authenticator app.
const TOTPIssuer = "Loadout"

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func newTOTPSecret() []byte {
	b := make([]byte, totpSecretLen)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error
	return b
}

// totpStep is the 30-second step t falls in.
func totpStep(t time.Time) int64 {
	return t.Unix() / totpPeriod
}

// totpCode is the code for one step (RFC 4226 HOTP with the step as the counter).
func totpCode(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, n%1_000_000)
}

// matchTOTP returns the step that code is valid for at now, within the skew, and whether there
// was one. Steps at or before after are refused: that code was already used.
func matchTOTP(secret []byte, code string, now time.Time, after int64) (int64, bool) {
	if len(code) != totpDigits {
		return 0, false
	}
	cur := totpStep(now)
	for step := cur - totpSkew; step <= cur+totpSkew; step++ {
		if step > after && hmac.Equal([]byte(totpCode(secret, step)), []byte(code)) {
			return step, true
		}
	}
	return 0, false
}

// totpURI is the otpauth:// URI an authenticator app reads from a QR code.
func totpURI(secret []byte, email string) string {
	label := url.PathEscape(TOTPIssuer + ":" + email)
	q := url.Values{
		"secret":    {b32.EncodeToString(secret)},
		"issuer":    {TOTPIssuer},
		"algorithm": {"SHA1"},
		"digits":    {fmt.Sprint(totpDigits)},
		"period":    {fmt.Sprint(totpPeriod)},
	}
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// TOTPKeyLen is the length of the key that encrypts TOTP secrets: AES-256.
const TOTPKeyLen = 32

var errBadCiphertext = errors.New("auth: can't decrypt TOTP secret")

// sealer encrypts TOTP secrets with AES-256-GCM. The user's ID is the additional data, so a
// secret copied onto another user's row doesn't decrypt.
type sealer struct {
	aead cipher.AEAD
}

func newSealer(key []byte) (sealer, error) {
	if len(key) != TOTPKeyLen {
		return sealer{}, fmt.Errorf("auth: TOTP key is %d bytes, want %d", len(key), TOTPKeyLen)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return sealer{}, fmt.Errorf("auth: TOTP key: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return sealer{}, fmt.Errorf("auth: TOTP key: %w", err)
	}
	return sealer{aead: aead}, nil
}

// seal returns nonce || ciphertext.
func (s sealer) seal(plain, ad []byte) []byte {
	nonce := make([]byte, s.aead.NonceSize())
	_, _ = rand.Read(nonce) // crypto/rand.Read never returns an error
	return s.aead.Seal(nonce, nonce, plain, ad)
}

func (s sealer) open(sealed, ad []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(sealed) < n {
		return nil, errBadCiphertext
	}
	plain, err := s.aead.Open(nil, sealed[:n], sealed[n:], ad)
	if err != nil {
		return nil, errBadCiphertext
	}
	return plain, nil
}

// Recovery codes: 16 base32 characters (80 bits) in groups of four, e.g. "k7qm-2xph-a9tc-w4nd".
const (
	recoveryCodeCount = 10
	recoveryCodeLen   = 16
)

func newRecoveryCodes() []string {
	codes := make([]string, recoveryCodeCount)
	for i := range codes {
		b := make([]byte, recoveryCodeLen*5/8)
		_, _ = rand.Read(b) // crypto/rand.Read never returns an error
		s := strings.ToLower(b32.EncodeToString(b))
		codes[i] = s[0:4] + "-" + s[4:8] + "-" + s[8:12] + "-" + s[12:16]
	}
	return codes
}

// normalizeRecoveryCode accepts a code as typed: any case, with or without dashes and spaces.
// It returns "" for anything that can't be one.
func normalizeRecoveryCode(code string) string {
	code = strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(code))
	if len(code) != recoveryCodeLen {
		return ""
	}
	if _, err := b32.DecodeString(strings.ToUpper(code)); err != nil {
		return ""
	}
	return code
}

func hashRecoveryCode(normalized string) []byte {
	h := sha256.Sum256([]byte(normalized))
	return h[:]
}

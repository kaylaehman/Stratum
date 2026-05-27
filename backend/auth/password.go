// Package auth provides authentication primitives: bcrypt password hashing,
// HS256 JWT issue/verify, and refresh-token generation/hashing. DB-backed
// session rotation is orchestrated by the API layer using these primitives.
package auth

import "golang.org/x/crypto/bcrypt"

// dummyBcryptHash is a real bcrypt hash (default cost) used by DummyCompare to
// equalize login timing when a username is not found, preventing user
// enumeration via response-time differences.
var dummyBcryptHash, _ = bcrypt.GenerateFromPassword([]byte("stratum-dummy-no-such-user"), bcrypt.DefaultCost)

// DummyCompare runs a bcrypt comparison against a fixed dummy hash. Call it on
// the user-not-found path so that path spends the same ~bcrypt time as a
// wrong-password path.
func DummyCompare(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(password))
}

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword reports whether plain matches the bcrypt hash. A nil return
// means the password is correct.
func CheckPassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}

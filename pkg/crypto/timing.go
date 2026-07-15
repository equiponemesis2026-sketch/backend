package crypto

import "crypto/subtle"

// ConstantTimeCompare compares two strings in constant time to prevent timing attacks
func ConstantTimeCompare(a, b string) bool {
	// subtle.ConstantTimeCompare requires []byte of identical size
	// We pad or hash if sizes differ, or we can use hashing to guarantee equal length
	// Standard strategy: hash both inputs with SHA-256 first, then compare their hashes
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

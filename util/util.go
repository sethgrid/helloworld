// util is considered an anti-pattern in Go. Sorry purists.
package util

import (
	"math/rand"
)

// no vowels to prevent accidental curse words, use url safe characters
const charset = "bcdfghjklmnpqrstuvwxyzBCDFGHJKLMNPQRSTVWXYZ0123456789-_:.$+!*"

// GenerateRandomString of a given length. Negaitive will return an empty string
func GenerateRandomString(length int) string {
	var result string
	for i := 0; i < length; i++ {
		result += string(charset[rand.Intn(len(charset))])
	}

	return result
}

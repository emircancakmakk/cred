// Package password generates random passwords for credential fields.
package password

import (
	"crypto/rand"
	"errors"
	"io"
	"strings"
)

const Length = 24

const (
	lower    = "abcdefghijklmnopqrstuvwxyz"
	upper    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits   = "0123456789"
	symbols  = "!@#$%^&*()-_=+"
	alphabet = lower + upper + digits + symbols
)

// Generate returns a password containing at least one character from each group.
func Generate() (string, error) { return generate(rand.Reader) }

func generate(source io.Reader) (string, error) {
	// Reject out-of-range bytes, then reject candidates missing a group. This
	// samples uniformly from passwords satisfying the four-group requirement.
	const limit = 256 - 256%len(alphabet)
	var candidate [Length]byte
	var sample [1]byte
	for {
		for i := range candidate {
			for {
				if _, err := io.ReadFull(source, sample[:]); err != nil {
					return "", errors.New("could not generate password")
				}
				if int(sample[0]) < limit {
					candidate[i] = alphabet[int(sample[0])%len(alphabet)]
					break
				}
			}
		}
		value := string(candidate[:])
		if strings.ContainsAny(value, lower) && strings.ContainsAny(value, upper) && strings.ContainsAny(value, digits) && strings.ContainsAny(value, symbols) {
			return value, nil
		}
	}
}

package vault

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode"
)

var (
	ErrInvalidEmail  = errors.New("enter a plain email address without a display name, quotes or whitespace")
	ErrEmailExists   = errors.New("email address already saved")
	ErrEmailNotFound = errors.New("saved email address not found")
)

// EmailBatch is a review of a batch, in input order. Invalid entries prevent all writes.
type EmailBatch struct{ Added, Skipped, Invalid []string }

func NormalizeEmail(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" || strings.ContainsAny(s, "\"\\<>()") || strings.ContainsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return "", ErrInvalidEmail
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s {
		return "", ErrInvalidEmail
	}
	at := strings.LastIndexByte(s, '@')
	if at <= 0 {
		return "", ErrInvalidEmail
	}
	return s[:at+1] + strings.ToLower(s[at+1:]), nil
}

func PreviewEmails(raw string, existing []string) EmailBatch {
	seen := make(map[string]bool)
	for _, s := range existing {
		seen[emailKey(s)] = true
	}
	var batch EmailBatch
	for _, token := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' || r == ';' }) {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		address, err := NormalizeEmail(token)
		if err != nil {
			batch.Invalid = append(batch.Invalid, token)
			continue
		}
		key := emailKey(address)
		if seen[key] {
			batch.Skipped = append(batch.Skipped, address)
			continue
		}
		seen[key] = true
		batch.Added = append(batch.Added, address)
	}
	return batch
}

func (v *Vault) AddSavedEmails(raw string) (EmailBatch, error) {
	if v == nil {
		return EmailBatch{}, errNilVault
	}
	existing := make([]string, 0, len(v.SavedEmails))
	for _, s := range v.SavedEmails {
		existing = append(existing, s)
	}
	batch := PreviewEmails(raw, existing)
	if len(batch.Invalid) > 0 {
		return batch, fmt.Errorf("%d invalid email entries; nothing saved", len(batch.Invalid))
	}
	// Allocate every ID before changing the vault, including on randomness failure.
	pending := make(map[string]string)
	for _, address := range batch.Added {
		for {
			var buf [16]byte
			if _, err := rand.Read(buf[:]); err != nil {
				return batch, errors.New("could not create saved email; try again")
			}
			id := hex.EncodeToString(buf[:])
			if _, ok := v.SavedEmails[id]; ok {
				continue
			}
			if _, ok := pending[id]; ok {
				continue
			}
			pending[id] = address
			break
		}
	}
	if len(pending) == 0 {
		return batch, nil
	}
	if v.SavedEmails == nil {
		v.SavedEmails = make(map[string]string)
	}
	for id, address := range pending {
		v.SavedEmails[id] = address
	}
	v.Version = currentVersion
	return batch, nil
}

func (v *Vault) EditSavedEmail(id, input string) error {
	if v == nil {
		return errNilVault
	}
	if _, ok := v.SavedEmails[id]; !ok {
		return ErrEmailNotFound
	}
	address, err := NormalizeEmail(input)
	if err != nil {
		return err
	}
	for other, s := range v.SavedEmails {
		if other != id && strings.EqualFold(s, address) {
			return ErrEmailExists
		}
	}
	v.SavedEmails[id] = address
	v.Version = currentVersion
	return nil
}

func (v *Vault) DeleteSavedEmail(id string) error {
	if v == nil {
		return errNilVault
	}
	if _, ok := v.SavedEmails[id]; !ok {
		return ErrEmailNotFound
	}
	delete(v.SavedEmails, id)
	v.Version = currentVersion
	return nil
}

func validVersion(version int) bool { return version == legacyVersion || version == currentVersion }

func validateSavedEmails(v *Vault) error {
	if v.Version == legacyVersion && len(v.SavedEmails) > 0 {
		return ErrCorruptVault
	}
	seen := make(map[string]bool)
	for id, address := range v.SavedEmails {
		decoded, err := hex.DecodeString(id)
		if err != nil || len(decoded) != 16 || strings.ToLower(id) != id {
			return ErrCorruptVault
		}
		normalized, err := NormalizeEmail(address)
		key := emailKey(address)
		if err != nil || normalized != address || seen[key] {
			return ErrCorruptVault
		}
		seen[key] = true
	}
	return nil
}

// Use the same Unicode folding equivalence as EqualFold, including final sigma.
func emailKey(address string) string {
	return strings.Map(func(r rune) rune {
		smallest := r
		for folded := unicode.SimpleFold(r); folded != r; folded = unicode.SimpleFold(folded) {
			if folded < smallest {
				smallest = folded
			}
		}
		return smallest
	}, address)
}

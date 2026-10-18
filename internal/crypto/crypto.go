package crypto

import (
	"bytes"
	"errors"
	"io"
	"runtime"

	"filippo.io/age"
)

const (
	MinScryptLogN = 1
	MaxScryptLogN = 22
)

var (
	ErrWrongPassword = errors.New("wrong master password")

	ErrCorruptVault = errors.New("vault file is corrupt or not a cred vault")

	ErrEmptyPassword = errors.New("master password is empty")

	ErrInvalidWorkFactor = errors.New("invalid scrypt work factor")
)

func Encrypt(plaintext, password []byte, workFactor int) ([]byte, error) {
	if len(password) == 0 {
		return nil, ErrEmptyPassword
	}
	if workFactor != 0 && (workFactor < MinScryptLogN || workFactor > MaxScryptLogN) {
		return nil, ErrInvalidWorkFactor
	}

	recipient, err := age.NewScryptRecipient(string(password))
	if err != nil {
		return nil, ErrEmptyPassword
	}
	if workFactor != 0 {
		recipient.SetWorkFactor(workFactor)
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return nil, ErrCorruptVault
	}
	if _, err := w.Write(plaintext); err != nil {
		_ = w.Close()
		Wipe(buf.Bytes())
		return nil, ErrCorruptVault
	}
	if err := w.Close(); err != nil {
		Wipe(buf.Bytes())
		return nil, ErrCorruptVault
	}

	out := bytes.Clone(buf.Bytes())
	Wipe(buf.Bytes())
	return out, nil
}

func Decrypt(ciphertext, password []byte) ([]byte, error) {
	if len(password) == 0 {
		return nil, ErrEmptyPassword
	}

	identity, err := age.NewScryptIdentity(string(password))
	if err != nil {
		return nil, ErrEmptyPassword
	}
	reader, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		if errors.Is(err, age.ErrIncorrectIdentity) {
			return nil, ErrWrongPassword
		}
		return nil, ErrCorruptVault
	}
	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}

	plain, err := io.ReadAll(reader)
	if err != nil {
		Wipe(plain)
		return nil, ErrCorruptVault
	}
	return plain, nil
}

func Wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}

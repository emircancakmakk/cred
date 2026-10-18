package crypto

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	password := []byte("test-password")
	tests := []struct {
		name      string
		plaintext []byte
	}{
		{name: "empty", plaintext: nil},
		{name: "short", plaintext: []byte("hello")},
		{name: "json", plaintext: []byte(`{"version":1,"projects":{}}`)},
		{name: "unicode", plaintext: []byte("şifre-değer")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sealed, err := Encrypt(tt.plaintext, password, MinScryptLogN)
			if err != nil {
				t.Fatalf("encrypt: %v", err)
			}
			if bytes.Contains(sealed, tt.plaintext) && len(tt.plaintext) > 0 {
				t.Fatal("ciphertext contains plaintext")
			}
			got, err := Decrypt(sealed, password)
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			defer Wipe(got)
			if !bytes.Equal(got, tt.plaintext) {
				t.Fatalf("plaintext mismatch: got %q", got)
			}
		})
	}
}

func TestEncryptDefaultWorkFactor(t *testing.T) {
	password := []byte("test-password")
	plaintext := []byte("default-work-factor")
	sealed, err := Encrypt(plaintext, password, 0)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	got, err := Decrypt(sealed, password)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	defer Wipe(got)
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext mismatch: got %q", got)
	}
}

func TestDecryptWrongPassword(t *testing.T) {
	password := []byte("super-secret-password-value")
	plaintext := []byte("plaintext-secret-value")
	sealed, err := Encrypt(plaintext, password, MinScryptLogN)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Decrypt(sealed, []byte("other-password-value"))
	if !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("got %v, want wrong password", err)
	}
	if errors.Is(err, ErrCorruptVault) {
		t.Fatal("wrong password reported as corrupt")
	}
	assertNoSecret(t, err, string(password), string(plaintext))
}

func TestDecryptCorrupt(t *testing.T) {
	password := []byte("super-secret-password-value")
	plaintext := []byte("plaintext-secret-value")
	sealed, err := Encrypt(plaintext, password, MinScryptLogN)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		in   []byte
	}{
		{name: "garbage", in: []byte("this is not an age file")},
		{name: "truncated", in: sealed[:len(sealed)/2]},
		{name: "empty", in: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(tt.in, password)
			if !errors.Is(err, ErrCorruptVault) {
				t.Fatalf("got %v, want corrupt", err)
			}
			if errors.Is(err, ErrWrongPassword) {
				t.Fatal("corrupt file reported as wrong password")
			}
			assertNoSecret(t, err, string(password), string(plaintext))
		})
	}
}

func TestEncryptRejectsEmptyPassword(t *testing.T) {
	_, err := Encrypt([]byte("data"), nil, MinScryptLogN)
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("encrypt got %v", err)
	}
	_, err = Decrypt([]byte("data"), nil)
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("decrypt got %v", err)
	}
}

func TestEncryptRejectsWorkFactorWithoutPanic(t *testing.T) {
	for _, n := range []int{-1, 23, 30, 31} {
		_, err := Encrypt([]byte("data"), []byte("password"), n)
		if !errors.Is(err, ErrInvalidWorkFactor) {
			t.Fatalf("work factor %d: got %v", n, err)
		}
	}
}

func TestWipe(t *testing.T) {
	b := []byte("secret-value")
	Wipe(b)
	if !bytes.Equal(b, make([]byte, len("secret-value"))) {
		t.Fatal("buffer was not wiped")
	}
}

func assertNoSecret(t *testing.T, err error, secrets ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(msg, secret) {
			t.Fatal("error contains secret material")
		}
	}
}

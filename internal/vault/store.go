package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/emircancakmakk/cred/internal/crypto"
)

const (
	envVaultPath = "CRED_VAULT"
	configDir    = "cred"
	vaultFile    = "vault.age"
)

type Options struct {
	WorkFactor int
}

func Path() (string, error) {
	if override := os.Getenv(envVaultPath); override != "" {
		return override, nil
	}
	return DefaultPath()
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate config directory: %w", err)
	}
	return filepath.Join(dir, configDir, vaultFile), nil
}

func Load(path string, password []byte) (*Vault, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrVaultNotFound
		}
		return nil, fmt.Errorf("read vault: %w", err)
	}

	plain, err := crypto.Decrypt(data, password)
	crypto.Wipe(data)
	if err != nil {
		return nil, mapDecryptError(err)
	}
	defer crypto.Wipe(plain)

	return parsePlaintext(plain)
}

func Save(path string, password []byte, v *Vault, opt Options) error {
	if v == nil {
		return errNilVault
	}
	if path == "" {
		return errors.New("vault path is empty")
	}

	plain, err := marshalVault(v)
	if err != nil {
		return err
	}
	defer crypto.Wipe(plain)

	sealed, err := crypto.Encrypt(plain, password, opt.WorkFactor)
	if err != nil {
		return mapEncryptError(err)
	}
	defer crypto.Wipe(sealed)

	dir := filepath.Dir(path)
	if err := ensureDir(dir); err != nil {
		return err
	}
	if err := backupExisting(path); err != nil {
		return err
	}
	return writeNewFile(dir, path, sealed)
}

func marshalVault(v *Vault) ([]byte, error) {
	if v.Version != currentVersion {
		return nil, ErrUnsupportedVersion
	}
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode vault: %w", err)
	}
	buf = append(buf, '\n')
	return buf, nil
}

func parsePlaintext(plain []byte) (*Vault, error) {
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(plain, &header); err != nil {
		return nil, ErrCorruptVault
	}
	if header.Version != currentVersion {
		return nil, ErrUnsupportedVersion
	}

	dec := json.NewDecoder(bytes.NewReader(plain))
	dec.DisallowUnknownFields()
	var loaded Vault
	if err := dec.Decode(&loaded); err != nil {
		return nil, ErrCorruptVault
	}
	if dec.More() {
		return nil, ErrCorruptVault
	}
	loaded.normalize()
	return &loaded, nil
}

func mapDecryptError(err error) error {
	switch {
	case errors.Is(err, crypto.ErrWrongPassword):
		return ErrWrongPassword
	case errors.Is(err, crypto.ErrEmptyPassword):
		return ErrEmptyPassword
	default:
		return ErrCorruptVault
	}
}

func mapEncryptError(err error) error {
	switch {
	case errors.Is(err, crypto.ErrEmptyPassword):
		return ErrEmptyPassword
	case errors.Is(err, crypto.ErrInvalidWorkFactor):
		return err
	default:
		return fmt.Errorf("encrypt vault: %w", err)
	}
}

func ensureDir(dir string) error {
	dir = filepath.Clean(dir)
	if info, err := os.Stat(dir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("vault parent %s is not a directory", dir)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat vault directory: %w", err)
	}

	var created []string
	cur := dir
	for {
		_, err := os.Stat(cur)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat vault directory: %w", err)
		}
		created = append(created, cur)
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create vault directory: %w", err)
	}
	for _, path := range created {
		if err := os.Chmod(path, 0o700); err != nil {
			return fmt.Errorf("set vault directory permissions: %w", err)
		}
	}
	return nil
}

func backupExisting(path string) error {
	src, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open vault for backup: %w", err)
	}
	defer src.Close()

	bak := path + ".bak"
	dst, err := os.OpenFile(bak, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create vault backup: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return fmt.Errorf("write vault backup: %w", err)
	}
	if err := dst.Sync(); err != nil {
		_ = dst.Close()
		return fmt.Errorf("sync vault backup: %w", err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("close vault backup: %w", err)
	}
	if err := os.Chmod(bak, 0o600); err != nil {
		return fmt.Errorf("set vault backup permissions: %w", err)
	}
	return nil
}

func writeNewFile(dir, path string, data []byte) error {
	tmp, err := os.CreateTemp(dir, ".vault-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary vault file: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary vault file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary vault file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary vault file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace vault file: %w", err)
	}
	committed = true
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("set vault file permissions: %w", err)
	}
	return syncDir(dir)
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open vault directory: %w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("sync vault directory: %w", err)
	}
	return nil
}

package clipboard

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/emircancakmakk/cred/internal/crypto"
)

const (
	ClearCommand = "__clipboard_clear"

	DefaultClearAfter = 30 * time.Second

	maxClearBytes = 1 << 20
)

var clearAfter = DefaultClearAfter

type Mac struct{}

func (Mac) Copy(text string) error {
	return Copy(text)
}

func Copy(text string) error {
	buf := []byte(text)
	defer crypto.Wipe(buf)
	if err := writeClipboard(buf); err != nil {
		return err
	}
	return startClearer(buf, clearAfter)
}

func HandleClearArgs(args []string, stdin io.Reader) (bool, error) {
	if len(args) == 0 || args[0] != ClearCommand {
		return false, nil
	}
	wait := DefaultClearAfter
	if len(args) > 1 {
		parsed, err := time.ParseDuration(args[1])
		if err != nil {
			return true, errors.New("invalid clipboard clear delay")
		}
		wait = parsed
	}
	if err := RunClearer(stdin, wait); err != nil {
		return true, err
	}
	return true, nil
}

func RunClearer(r io.Reader, wait time.Duration) error {
	secret, err := io.ReadAll(io.LimitReader(r, maxClearBytes+1))
	if err != nil {
		return errors.New("read clipboard clear input")
	}
	defer crypto.Wipe(secret)
	if len(secret) > maxClearBytes {
		return errors.New("clipboard clear input is too large")
	}
	if wait > 0 {
		time.Sleep(wait)
	}
	current, err := paste()
	if err != nil {
		return err
	}
	defer crypto.Wipe(current)
	if subtle.ConstantTimeCompare(current, secret) != 1 {
		return nil
	}
	return writeClipboard(nil)
}

func writeClipboard(data []byte) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = bytes.NewReader(data)
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("copy to clipboard: pbcopy not found")
		}
		return errors.New("copy to clipboard")
	}
	return nil
}

func paste() ([]byte, error) {
	cmd := exec.Command("pbpaste")
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(out) == 0 {
		return []byte{}, nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return nil, errors.New("read clipboard: pbpaste not found")
	}
	return nil, errors.New("read clipboard")
}

func startClearer(secret []byte, wait time.Duration) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	cmd := exec.Command(exe, ClearCommand, wait.String())
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	defer null.Close()
	cmd.Stdout = null
	cmd.Stderr = null

	reader, writer, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create clipboard pipe: %w", err)
	}
	cmd.Stdin = reader
	if err := cmd.Start(); err != nil {
		reader.Close()
		writer.Close()
		return errors.New("start clipboard clear")
	}
	reader.Close()
	_, writeErr := writer.Write(secret)
	closeErr := writer.Close()
	if writeErr != nil || closeErr != nil {
		_ = cmd.Process.Kill()
		return errors.New("start clipboard clear")
	}

	return cmd.Process.Release()
}

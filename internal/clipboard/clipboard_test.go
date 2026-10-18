package clipboard

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emircancakmakk/cred/internal/crypto"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "__copy_and_exit" {
		os.Exit(runCopyAndExit())
	}
	if len(os.Args) > 1 && os.Args[1] == ClearCommand {
		if logPath := os.Getenv("CRED_ARGV_LOG"); logPath != "" {
			_ = os.WriteFile(logPath, []byte(strings.Join(os.Args, "\n")), 0o600)
		}
		handled, err := HandleClearArgs(os.Args[1:], os.Stdin)
		if !handled || err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runCopyAndExit() int {
	if len(os.Args) < 3 {
		return 2
	}
	wait, err := time.ParseDuration(os.Args[2])
	if err != nil {
		return 2
	}
	clearAfter = wait
	text, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 1
	}
	defer crypto.Wipe(text)
	if err := Copy(string(text)); err != nil {
		return 1
	}
	return 0
}

func TestHandleClearArgsRejectsBadDelay(t *testing.T) {
	secret := "DELAY-SECRET-VALUE"
	handled, err := HandleClearArgs([]string{ClearCommand, secret}, bytes.NewReader([]byte(secret)))
	if !handled {
		t.Fatal("expected the clear command to be handled")
	}
	if err == nil || err.Error() != "invalid clipboard clear delay" {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("error contains secret material")
	}
}

func TestRunClearer(t *testing.T) {
	tests := []struct {
		name   string
		start  string
		secret string
		want   string
	}{
		{name: "same value is cleared", start: "same-secret", secret: "same-secret", want: ""},
		{name: "different value stays", start: "user-copied", secret: "same-secret", want: "user-copied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clip := installFakeClipboard(t)
			if err := os.WriteFile(clip, []byte(tt.start), 0o600); err != nil {
				t.Fatal(err)
			}
			err := RunClearer(bytes.NewReader([]byte(tt.secret)), 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := readClip(t, clip); got != tt.want {
				t.Fatalf("clipboard = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunClearerHidesSecretOnError(t *testing.T) {
	secret := "HIDDEN-CLIPBOARD-SECRET"
	t.Setenv("PATH", t.TempDir())
	err := RunClearer(bytes.NewReader([]byte(secret)), 0)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("error contains secret material")
	}
}

func TestCopyErrorHidesSecret(t *testing.T) {
	secret := "HIDDEN-CLIPBOARD-SECRET"
	t.Setenv("PATH", t.TempDir())
	err := Copy(secret)
	if err == nil || !strings.Contains(err.Error(), "pbcopy not found") {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("error contains secret material")
	}
}

func TestClearSurvivesParentExit(t *testing.T) {
	clip := installFakeClipboard(t)
	logPath := filepath.Join(t.TempDir(), "argv")
	t.Setenv("CRED_ARGV_LOG", logPath)
	secret := "DETACHED-SECRET-VALUE"
	if err := runCopyHelper(t, secret, 300*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if got := readClip(t, clip); got != secret {
		t.Fatalf("clipboard after parent exit = %q", got)
	}
	waitForClip(t, clip, "", 3*time.Second)
	log := readClip(t, logPath)
	if strings.Contains(log, secret) {
		t.Fatal("clear process arguments contain the secret")
	}
	if !strings.Contains(log, ClearCommand) {
		t.Fatalf("argv log = %q", log)
	}
}

func TestClearLeavesReplacedClipboard(t *testing.T) {
	clip := installFakeClipboard(t)
	secret := "DETACHED-SECRET-VALUE"
	if err := runCopyHelper(t, secret, 400*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	const replaced = "user-copied-something-else"
	if err := os.WriteFile(clip, []byte(replaced), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(700 * time.Millisecond)
	if got := readClip(t, clip); got != replaced {
		t.Fatalf("clipboard = %q", got)
	}
}

func runCopyHelper(t *testing.T, secret string, wait time.Duration) error {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "__copy_and_exit", wait.String())
	cmd.Stdin = strings.NewReader(secret)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func installFakeClipboard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	clip := filepath.Join(dir, "clip")
	writeExe(t, filepath.Join(bin, "pbcopy"), "#!/bin/sh\n/bin/cat > "+shellQuote(clip)+"\n")
	writeExe(t, filepath.Join(bin, "pbpaste"), "#!/bin/sh\nif [ -f "+shellQuote(clip)+" ]; then /bin/cat "+shellQuote(clip)+"; fi\n")
	t.Setenv("PATH", bin+":/bin:/usr/bin")
	return clip
}

func writeExe(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

func readClip(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func waitForClip(t *testing.T, path, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if readClip(t, path) == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("clipboard = %q, want %q", readClip(t, path), want)
}

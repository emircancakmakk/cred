package cmd

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/emircancakmakk/cred/internal/terminalui"
	"github.com/emircancakmakk/cred/internal/vault"
	"golang.org/x/term"
)

// startProgress serializes drawing and cleanup; stop is idempotent and waits
// until the drawing goroutine has exited before a prompt can be printed.
func startProgress(out io.Writer, enabled bool, label string) func() {
	if !enabled {
		return func() {}
	}
	done, finished := make(chan struct{}), make(chan struct{})
	var once sync.Once
	fmt.Fprintf(out, "\r\x1b[2K⠋ %s", terminalui.Message(label, terminalui.Accent))
	go func() {
		defer close(finished)
		ticker := time.NewTicker(90 * time.Millisecond)
		defer ticker.Stop()
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 1
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Fprintf(out, "\r\x1b[2K%s %s", frames[i%len(frames)], terminalui.Message(label, terminalui.Accent))
				i++
			}
		}
	}()
	return func() { once.Do(func() { close(done); <-finished; fmt.Fprint(out, "\r\x1b[2K") }) }
}
func (a *App) stage(label string) func() {
	if a.progressObserver != nil {
		return a.progressObserver(label)
	}
	out := a.errWriter()
	f, ok := out.(*os.File)
	return startProgress(out, ok && term.IsTerminal(int(f.Fd())), label)
}
func (a *App) loadVault(path string, password []byte) (*vault.Vault, error) {
	stop := a.stage("Unlocking vault")
	defer stop()
	return vault.Load(path, password)
}
func (a *App) mergeVault(base, local, remote *vault.Vault) (*vault.Vault, error) {
	stop := a.stage("Merging changes")
	defer func() { stop() }()
	return vault.Reconcile(base, local, remote, func(path string) (bool, error) {
		stop()
		mine, err := a.resolveConflict(path)
		if err == nil {
			stop = a.stage("Merging changes")
		}
		return mine, err
	})
}

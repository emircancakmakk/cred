package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/emircancakmakk/cred/internal/version"
)

func TestVersionDoesNotOpenVault(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-v"}, {"--vault", "does-not-exist", "--version"}} {
		var out, errout bytes.Buffer
		app := NewApp(strings.NewReader(""), &out, &errout)
		if e := app.Execute(args); e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(out.String(), version.Version) || errout.Len() != 0 {
			t.Fatalf("version output %q stderr %q", out.String(), errout.String())
		}
	}
}

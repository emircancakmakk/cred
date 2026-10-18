package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/emircancakmakk/cred/internal/clipboard"
	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/vault"
	"github.com/emircancakmakk/cred/internal/version"
)

type Copier interface {
	Copy(text string) error
}

type App struct {
	In         io.Reader
	Out        io.Writer
	Err        io.Writer
	Prompter   Prompter
	Copier     Copier
	WorkFactor int

	vaultName        string
	profile          profiles.Profile
	sessionPath      string
	gitSession       *profiles.Git
	base             *vault.Vault
	release          func()
	progressObserver profiles.Observer
	setupMenu        func() (setupChoice, error)

	RunTUI func(project string) error
}

func NewApp(in io.Reader, out, err io.Writer) *App {
	return &App{In: in, Out: out, Err: err, Copier: clipboard.Mac{}}
}

func (a *App) Execute(args []string) error {
	if args == nil {
		args = []string{}
	}
	a.vaultName = ""
	a.sessionPath = ""
	a.base = nil
	a.gitSession = nil
	a.release = nil
	defer func() {
		if a.release != nil {
			a.release()
		}
	}()
	// Extract the global selector before shorthand command rewriting.
	filtered := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			filtered = append(filtered, args[i:]...)
			break
		}
		if args[i] == "--vault" {
			if i+1 >= len(args) {
				return errors.New("--vault requires a profile")
			}
			i++
			a.vaultName = args[i]
			continue
		}
		if strings.HasPrefix(args[i], "--vault=") {
			a.vaultName = strings.TrimPrefix(args[i], "--vault=")
			continue
		}
		filtered = append(filtered, args[i])
	}
	args, err := rewriteArgs(filtered)
	if err != nil {
		return err
	}
	root := a.root()
	root.SetOut(a.outWriter())
	root.SetErr(a.errWriter())
	root.SetArgs(args)
	return root.Execute()
}

func (a *App) root() *cobra.Command {
	root := &cobra.Command{
		Use:     "cred",
		Version: version.Version,
		Short:   "Local encrypted credential vault",
		Long: `Local encrypted credential vault.

cred with no arguments opens the project list.
cred <project> opens that project's service list. Enter moves forward,
Esc or left moves back, and q quits. s shows a secret for 5 seconds.
/ filters the current list.

cred <project> <service> <field> copies one field without the selector.
Matching tries an exact name, then case-insensitive, then a unique prefix.
--show writes the value to stdout with a trailing newline and does not copy.`,
		Example: `  cred
  cred my-project
  cred my-project vercel password
  cred my-project vercel password --show`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.runTUI(nil, nil)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}
	root.PersistentFlags().String("vault", "", "select a vault profile")
	root.AddCommand(
		quiet(a.connectCmd()), quiet(a.vaultCmd()), quiet(a.statusCmd()),
		quiet(a.initCmd()),
		quiet(a.addCmd()),
		quiet(a.importCmd()),
		quiet(a.setCmd()),
		quiet(a.lsCmd()),
		quiet(a.rmCmd()),
		quiet(a.copyCmd()),
		quiet(a.tuiCmd()),
	)
	return root
}

func quiet(cmd *cobra.Command) *cobra.Command {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}

func (a *App) prompter() (Prompter, error) {
	if a.Prompter != nil {
		return a.Prompter, nil
	}
	file, ok := a.In.(*os.File)
	if !ok || file == nil {
		return nil, errors.New("interactive terminal required")
	}
	return newTerminalPrompter(file, a.errWriter()), nil
}

func (a *App) unlock() (*vault.Vault, string, []byte, error) {
	prompter, err := a.prompter()
	if err != nil {
		return nil, "", nil, err
	}
	path, err := a.prepare()
	if err != nil {
		return nil, "", nil, err
	}
	password, err := prompter.ReadPassword(promptPassword)
	if err != nil {
		return nil, "", nil, err
	}

	loaded, err := a.loadVault(path, password)
	if err != nil {
		crypto.Wipe(password)
		if errors.Is(err, vault.ErrVaultNotFound) {
			return nil, "", nil, fmt.Errorf("%w (run cred init)", err)
		}
		return nil, "", nil, err
	}
	a.base = vault.Clone(loaded)
	return loaded, path, password, nil
}

func (a *App) copier() (Copier, error) {
	if a.Copier == nil {
		return nil, errors.New("clipboard is unavailable")
	}
	return a.Copier, nil
}

func (a *App) save(path string, password []byte, v *vault.Vault) error {
	if a.gitSession != nil {
		return a.saveGit(password, v)
	}
	stop := a.stage("Encrypting vault")
	defer stop()
	return vault.Save(path, password, v, vault.Options{WorkFactor: a.WorkFactor})
}

func (a *App) outWriter() io.Writer {
	if a.Out != nil {
		return a.Out
	}
	return os.Stdout
}

func (a *App) errWriter() io.Writer {
	if a.Err != nil {
		return a.Err
	}
	return os.Stderr
}

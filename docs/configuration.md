# Configuration and status

## Vault file

The default path comes from Go's user config directory:

- macOS: `~/Library/Application Support/cred/vaults/local/vault.age`
- Linux: `$XDG_CONFIG_HOME/cred/vaults/local/vault.age`, or `~/.config/cred/vaults/local/vault.age`

`CRED_VAULT` selects another file. The variable is a path. The master
password is never read from the environment.

```sh
export CRED_VAULT="$HOME/tmp/cred-demo/vault.age"
```

Directories created for the vault are mode `0700`. The vault file and its
backup are mode `0600`.

## Status and progress

```sh
cred status                       # selected profile, local snapshot only
cred --vault personal status      # inspect a specific local profile
cred --vault team status          # Git checkout and pending changes
cred --vault team status --remote # additionally check remote access/commit
```

Status does not ask for the master password or open the encrypted vault. It shows
which profile was selected and why (`--vault`, `CRED_VAULT`, or default), the actual
vault path and whether its file exists. A missing vault is reported as not
initialized rather than created. Git profiles also show the checkout path, branch,
local commit, last fetch time, tracked working-copy changes, unpublished commits
relative to the last fetched refs, and the retained change/recovery base state, with retry/discard suggestions.
Unknown information is labeled as such. A busy profile is reported without
reading its in-progress Git state.

The default command is offline. `--remote` uses `git ls-remote` with your existing
Git credentials; it does not clone, fetch, reset, push, or change local refs. It
checks the tracked branch (or remote HEAD when no branch is known) and reports
whether its commit matches the local one. Different commits are reported without
an ahead/behind calculation. `--remote` requires a Git profile. Missing branches,
failed remote checks, unknown profiles and invalid configuration return an error;
ordinary status conditions such as missing files or pending changes do not.
A busy profile cannot run a remote check; retry after the other operation ends.

On an interactive stderr terminal, a single-line spinner labels cloning,
updating, unlocking, encrypting, merging and sharing phases. It stops and clears
before password/confirmation prompts and before the selector opens. Errors and
cancelled operations also clear it. No percentage is invented. Redirected stderr
has no progress output or animation, and `--show` stdout stays value-only.


[← README](../README.md) · [Documentation](README.md)

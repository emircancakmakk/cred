<p align="center">
  <img src="assets/cred-icon.png" alt="" width="96" height="96" />
</p>

<h1 align="center">cred</h1>

<p align="center">
  An encrypted credential vault for the terminal, with local and shared Git profiles.
</p>

<p align="center">
  <a href="https://github.com/emircancakmakk/cred/releases/latest"><img src="https://img.shields.io/github/v/release/emircancakmakk/cred?label=release&color=2eb857" alt="Latest release" /></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-000000" alt="macOS and Linux" />
  <img src="https://img.shields.io/badge/license-MIT-2eb857" alt="MIT license" />
</p>

Records are stored as project, then service, then field. One example is the
project `my-project`, with services `vercel` and `digital-ocean`, and fields
such as `email` and `password`.

The vault is a single file encrypted with [age](https://github.com/FiloSottile/age)
passphrase mode (scrypt). Local vaults work without a network. Optional Git profiles
synchronize an encrypted vault through a private repository using your existing
Git credentials.

## Install

With Homebrew:

```sh
brew install emircancakmakk/tap/cred
```

From source, Go 1.27.2 or newer, as declared in `go.mod`.

```sh
make build
```

This writes `./cred`. Put that binary on your `PATH`, or run it from the
repository with `go run .`.

```sh
make test
```

## Upgrade to v0.3.0

```sh
brew update
brew upgrade emircancakmakk/tap/cred
cred --version
```

This release adds named local/Git profiles, guided setup, conflict-aware Git
sharing, status checks and retry/discard recovery. See [CHANGELOG.md](CHANGELOG.md)
for the release notes.

The default local vault path has changed to `cred/vaults/local/vault.age` under
the user config directory. Existing files are **not** moved automatically. The
vault encryption, JSON version and master password remain compatible.

To keep using an existing macOS vault at its previous path:

```sh
export CRED_VAULT="$HOME/Library/Application Support/cred/vault.age"
cred
```

On Linux, point `CRED_VAULT` at the existing `$XDG_CONFIG_HOME/cred/vault.age` or
`~/.config/cred/vault.age`. Alternatively, copy the old file to the new default
location without overwriting an existing vault. No `cred init` is needed to open
an existing vault.

`cred init` now opens the interactive setup menu and creates named profiles,
even with `CRED_VAULT` set. Use `cred init personal` for direct local setup.
`connect`, `vault` and `status` are now reserved command names.

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

## Local and shared Git profiles

Git must be installed and authenticated (for example with an SSH key). Configure
`git config --global user.name` and `user.email` before writing to a Git vault.
Use a private repository and grant access individually through GitHub/GitLab.

```sh
# Guided setup: local, connect to a team, or create a new shared vault.
cred init

# Direct local setup (no menu).
cred init local

# Separate personal vault.
cred init personal
cred vault use personal

# First team member: create an empty private repository, then run cred init
# and choose "Create a shared vault in an empty Git repository".
# Enter the profile name, repository URL and a new master password.
# No intermediate local seed profile is needed.

# Other members: connect to the existing encrypted vault; no init required.
cred connect team git@github.com:company/team-vault.git

cred --vault team add my-project vercel
cred --vault team my-project vercel password
cred vault list
cred vault use team
cred vault use local  # select the local profile
```

`connect` asks for the existing vault's master password and validates it before
saving the connection. `--from` publishes only into an empty repository and
leaves the source local vault independent. New connections do not change the
default. Git profiles follow the remote's default branch. Each vault has its own
password. Machine-local settings live in `cred/profiles.json` in the user config
directory; passwords are never stored there.

`--vault NAME` selects a profile for one command. Without it, `CRED_VAULT` takes
precedence over the selected default and always means a local file. Explicit
`--vault local` selects the profile named `local`, independently of the environment
override. Names `connect`, `vault` and `status` are reserved commands. `local` is an ordinary
profile name; the local setup screen suggests `local`, while `cred init personal`
creates `personal` directly. There is no legacy vault location or automatic migration.
Parameterless `cred init` always opens the setup menu, even when `CRED_VAULT` is
set; the wizard creates named profiles rather than a file at the override path.

Local profiles never access Git. Local vaults live at `cred/vaults/<name>/vault.age`
inside the user config directory. Git profiles clone once into the private managed
checkout `cred/vaults/<name>/repo/` and reuse it. Each operation fetches and updates
to the remote branch before opening `repo/vault.age`; Git never merges the
encrypted file. Network access is required, including for reads. Uncommitted edits
or unpublished commits in the managed checkout stop normal operations; retained
changes are recovered with `cred vault retry`. Recovery may reset the managed
checkout after reading the encrypted recovery pair. Do not edit the managed repo
manually. Recovery files stay outside `repo/`, in the profile directory.
Only `vault.age` is staged by cred. Values are
decrypted only in memory, and commit messages contain no record names or values.
The active profile is displayed in the selector and on named/Git operations.

Concurrent writes use ordinary non-force pushes. On rejection, cred fetches the
latest version and compares the original, local and remote vaults. Different
fields merge automatically. A field's value and secret flag are one unit. Same
field conflicts and deletion versus descendant edits prompt for team, mine, or
cancel; the default is cancel and values are not displayed. Case-only name
collisions stop the merge. At most three publication attempts are made.
Local processes are locked for the entire read/modify/write session.

If publication fails or a merge is cancelled, cred retains encrypted change and
base files outside the repository and reports an error instead of success.
Further Git operations on that profile stop until you retry or discard:

```sh
cred vault retry team     # merge and publish the retained change
cred vault discard team   # abandon it and restore a validated remote snapshot
```

Retry merges the retained change with the current remote and asks again for any
conflicts. Only a concurrent remote update triggers automatic merge/retry. Other
Git failures stop immediately and retain the encrypted change, with a hint for
missing Git, connectivity/DNS, authentication/access, missing author identity,
timeout or server policy rejection. Unrecognized failures are identified as
unknown; raw Git diagnostics and credential-bearing repository URLs are not
printed. Git hooks are disabled in managed working copies. No offline edits or automatic fallback to local storage occur.

Git history retains old encrypted vaults. Removing a member's repository access
cannot revoke downloaded copies. Rotate the underlying service credentials when
necessary. This version has no master password rotation command or per-project
permissions; every member with repository access and the password can read the
whole vault. Require the same conflict-aware client for all writers and avoid
manual/force replacement of `vault.age`.

## Discard pending Git changes

```sh
cred vault discard team
```

Discard requires a Git profile and asks for confirmation, defaulting to no. There
is no flag to skip confirmation. Cancellation does not access the network or
change vault data. If there is no pending change, leftover recovery file or
incomplete setup, the command reports `nothing to discard` without asking for a
password or accessing the network.

After confirmation, cred checks the remote and fetches the selected branch. It
asks once for the **remote vault's** master password and validates the exact
fetched snapshot in memory before resetting the managed checkout. Wrong passwords,
corrupt or missing remote vaults, symbolic links, missing branches and connection
failures leave pending data intact. There is no offline fallback. The validated
commit is pinned, so a remote update during verification cannot replace it with
an unvalidated snapshot. No changes are pushed to the remote repository.

After restoring that commit, cred removes the pending change, its base and their
backups. It also removes the checkout's untracked `vault.age.bak`, preserving
tracked remote backups and unrelated untracked files. File cleanup errors are
reported instead of success; run the same discard command again to finish after
resolving the reported filesystem problem. The primary pending file is removed
last, and discard does not need the recovery base to be readable or present.

For an incomplete first setup, confirmation explicitly explains that a still-empty
remote means removing the unfinished **local** profile, checkout and recovery
files. This cancellation does not ask for a master password or modify the remote.
If a remote vault has since appeared, cred validates it with its current password
and restores it, keeping the profile and completing setup instead. A failed config
save rolls the moved local directory back. If final directory cleanup fails, the
error identifies where remaining data is stored. The selected default is preserved.

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

## Usage

`cred` with no arguments opens the project list. `cred --help` prints help.

### Create a vault

```sh
cred init
```

With no profile argument, `cred init` opens an interactive menu. Use the arrow
keys and Enter to select an option; Esc or Ctrl+C cancels the menu without
creating a profile. A terminal is required for both the menu and password input.

| Option | Prompts and behavior |
| --- | --- |
| Create a local vault | Profile name (empty means `local`), new master password twice |
| Connect to an existing team vault | Profile name, repository URL, existing master password once |
| Create a shared vault in an empty Git repository | Profile name, repository URL, new master password twice, first commit/push |

Create the empty private Git repository yourself, using the provider's web UI or
CLI. Do not add a README, license or `.gitignore`; any existing branch or tag
rejects new shared-vault setup. cred creates the encrypted `vault.age`, not the
repository or a provider account. Git authentication and author identity must
already be configured.

New master passwords must contain at least 12 Unicode characters. Existing
profiles and vault files are never overwritten, and failed password validation
does not register a profile. Opening an existing vault does not apply the length
rule again.

After successful guided setup, cred prints the profile, vault path and a usage
example, then asks `Use this vault by default? [y/N]:`. Only `y` or `yes` changes
the selected default. With `CRED_VAULT` set, explicitly select a profile or unset
the variable before relying on the default.

`cred init personal` (or `cred --vault personal init`) skips the menu and creates
a local profile directly, without a default-selection prompt. `cred connect team
<repository>` remains the direct connection shortcut. To publish an existing local
vault into an empty repository, use `cred connect team <repository> --from personal`;
the source local vault remains independent.

Before a shared vault's first push, cred persists encrypted recovery snapshots
and an `initializing` profile. A failed push never reports setup as completed or
changes the default. `cred status` reports `Setup: incomplete`; resume with:

```sh
cred vault retry team
```

To abandon that unfinished setup instead, use `cred vault discard team`. It asks
for confirmation and cancels the local setup if the remote is still empty.

Retry can complete the first push into a still-empty repository. If another user
has created the remote vault, retry merges against that vault using the normal
conflict rules; a different master password stops recovery and preserves the
snapshots. The setup marker and recovery files are cleared after success. Until
then, ordinary vault operations and selecting this profile as default are blocked.

### Add fields

```sh
cred add my-project vercel
cred add my-project
```

The service name is prompted when it is omitted. Each field asks for a name,
a hidden value, and whether the value is secret (`Secret? [y/N]`). An empty
field name ends the list. An empty secret answer means the field is not
secret. An empty value is allowed.

The file is written once, after every field in that run is accepted. If a
field name already exists, nothing from that run is saved. The message names
`cred set` for the existing field.

A new project, service, or field name that differs from an existing one only
by case is rejected.

### Import fields

```sh
cred import data.json
```

The file is JSON in the vault plaintext shape, version 1. The master password
is prompted once. The vault is written once, and only when at least one field
is new.

```json
{
  "version": 1,
  "projects": {
    "my-project": {
      "services": {
        "vercel": {
          "fields": {
            "email": { "value": "user@example.com", "secret": false },
            "password": { "value": "secret-value", "secret": true }
          }
        }
      }
    }
  }
}
```

`secret` is required on every field. Omitting it is an error, so a forgotten
flag cannot store a password as a visible field. `value` may be empty.
Unknown keys are rejected.

A field that is already stored under that exact name is skipped. A name that
differs only by case from an existing project, service, or field is skipped
too. The stored value is left as it is. Other fields in the file are still
saved. If every field is skipped, or the file contains no fields, the vault
file is left unchanged.

Skipped fields are listed on stderr as `project/service/field`. Values are
not printed. `added 1 field` and `added N fields` match `cred add`.

Two names in the file that differ only by case, a bad name, an unknown key,
or a version other than 1 rejects the file before the password prompt.
Nothing is saved.

A project or service with no fields does not create an empty record. The
import file is plaintext and is left on disk.

### Replace a field

```sh
cred set my-project vercel password
```

`set` changes the value and the secret flag of a field that already exists.
The value is prompted and hidden. On a secret field the prompt is
`Secret? [Y/n]`; an empty answer keeps the current flag. A missing field is
an error.

### List

```sh
cred ls
```

`ls` prints the project and service tree on stdout. Field names and values
are omitted. An empty vault prints `vault is empty`.

```text
my-project
  digital-ocean
  vercel
```

### Delete

```sh
cred rm my-project vercel password
cred rm my-project vercel
cred rm my-project
```

Every delete asks for confirmation. The prompt defaults to no: an empty
answer, `n`, or any answer other than `y` or `yes` leaves the file unchanged.
Deleting the last field leaves the service in place. Deleting the last
service leaves the project in place.

### Choose a field

```sh
cred
cred my-project
cred fil
```

`cred` opens the project list after the master password prompt. Each row
shows the project name and how many services it has. One name opens that
project's service list. The password is asked once; moving between screens
keeps the same unlocked snapshot.

Enter moves forward: project, then service, then field. Enter on a field
copies that value to the clipboard. Arrow keys, `j`, and `k` move. `/` filters
the current list. `s` shows the selected secret for 5 seconds, then masks it
again. Esc or left returns to the previous screen, including from a service
list opened with `cred <project>`. On the project list, Esc quits. `q` and
Ctrl+C quit from any screen.

Secret fields are shown as `••••••`. Other values are shown in plain text.
The status line names the field (`copied my-project/vercel/password`) and does not
include the value.

A single word that is not a command is treated as a project name. `cred ls`
lists the vault. `cred l` looks up a project named `l`.

### Copy one field directly

```sh
cred my-project vercel password
cred my-project vercel password --show
```

The first form copies the value and prints `copied my-project/vercel/password` on
stderr. `--show` writes the value to stdout with one trailing newline and
does not touch the clipboard. `--show` may appear before or after the names.

The clipboard keeps the value for 30 seconds. A detached process then clears
it when the pasteboard still holds that same value. A value you copy
afterward is left as it is.

## Name matching

`cred <project>` and `cred <project> <service> <field>` resolve names in this
order:

1. Exact name.
2. One name that matches ignoring case.
3. One name that starts with the query, ignoring case.
4. Several matches: an error that lists the candidate names. The selector
   does not open.

`add`, `set`, `rm`, and `import` require the stored spelling. A prefix is a
different name there. `import` skips a name that differs only by case from
one already stored.

Case comparison uses Unicode case folding (`strings.EqualFold`). `my-project` and
`MY-PROJECT` match. Turkish `i` / `İ` and `ı` / `I` stay distinct, so `ışık` and
`IŞIK` do not match.

Names must be non-empty, must not start or end with a space, and must not
contain `/`, `\`, or a control character. Spaces inside a name are kept.
Prompted names are stored as typed.

## Security model

- Encryption is `filippo.io/age` in scrypt passphrase mode. cred does not
  implement its own cipher. The file is binary age. The plaintext is JSON,
  version 1, holding projects, services, and fields.
- The master password is read from the terminal with echo off. It is not a
  command argument and it is not an environment variable. Password and secret
  bytes that cred controls are wiped after use.
- A wrong password and a damaged file are different errors. Those errors do
  not include the file body or any secret.
- Save encrypts in memory first. It then copies the current file to
  `vault.age.bak`, writes a temporary file in the same directory, syncs it,
  and renames it into place. If the backup fails, the original file is left
  unchanged. An existing directory is not given a new mode.
- Secret values are masked in `ls` and in the selector. They are written to
  stdout only with `--show`.
- The clipboard child receives the secret on a pipe. The value is not placed
  in the process arguments or the environment. The child clears the pasteboard
  only when it still contains that value.

## Known limits

- The clipboard integration is macOS `pbcopy` and `pbpaste`.
- Copied items are not marked with `org.nspasteboard.ConcealedType`. A
  clipboard manager can still display them. `pbcopy` cannot set that flag.
- age accepts the passphrase as a Go string. That copy cannot be wiped and
  remains until the garbage collector reclaims it.
- The default scrypt work factor is age's logN 18. Creating, opening, or
  saving a vault takes about a second.
- There is no command to change the master password, and no Touch ID,
  Keychain, export, or `cred env` command.
- The backup is a single previous generation, `vault.age.bak`.
- The selector's `s` key draws a secret on the alternate screen for 5
  seconds. That screen is separate from the normal scrollback.
- A command typed as one unknown word is resolved as a project name.

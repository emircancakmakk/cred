# cred

`cred` is a local encrypted credential vault for the terminal. Records are
stored as project, then service, then field. One example is the project
`my-project`, with services `vercel` and `digital-ocean`, and fields such as
`email` and `password`.

The vault is a single file encrypted with [age](https://github.com/FiloSottile/age)
passphrase mode (scrypt). There is no network sync, account, or GUI.

## Install

Go 1.27.2 or newer, as declared in `go.mod`.

```sh
make build
```

This writes `./cred`. Put that binary on your `PATH`, or run it from the
repository with `go run .`.

```sh
make test
```

## Vault file

The default path comes from Go's user config directory:

- macOS: `~/Library/Application Support/cred/vault.age`
- Linux: `$XDG_CONFIG_HOME/cred/vault.age`, or `~/.config/cred/vault.age`

`CRED_VAULT` selects another file. The variable is a path. The master
password is never read from the environment.

```sh
export CRED_VAULT="$HOME/tmp/cred-demo/vault.age"
```

Directories created for the vault are mode `0700`. The vault file and its
backup are mode `0600`.

## Usage

`cred` with no arguments opens the project list. `cred --help` prints help.

### Create a vault

```sh
cred init
```

The master password is prompted twice and hidden. It must be at least 12
Unicode characters. If the vault file already exists, `init` stops before
asking for a password.

Opening an existing vault does not apply the length rule again.

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

`add`, `set`, and `rm` require the stored spelling. A prefix is a different
name there.

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
  Keychain, sync, import, export, or `cred env` command.
- The backup is a single previous generation, `vault.age.bak`.
- The selector's `s` key draws a secret on the alternate screen for 5
  seconds. That screen is separate from the normal scrollback.
- A command typed as one unknown word is resolved as a project name.

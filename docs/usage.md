# Usage

`cred` with no arguments opens the project list. `cred --help` prints help.

## Create a vault

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

## Add fields

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

## Import fields

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

## Replace a field

```sh
cred set my-project vercel password
```

`set` changes the value and the secret flag of a field that already exists.
The value is prompted and hidden. On a secret field the prompt is
`Secret? [Y/n]`; an empty answer keeps the current flag. A missing field is
an error.

## List

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

## Delete

```sh
cred rm my-project vercel password
cred rm my-project vercel
cred rm my-project
```

Every delete asks for confirmation. The prompt defaults to no: an empty
answer, `n`, or any answer other than `y` or `yes` leaves the file unchanged.
Deleting the last field leaves the service in place. Deleting the last
service leaves the project in place.

## Choose a field

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

## Copy one field directly

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


[← README](../README.md) · [Documentation](README.md)

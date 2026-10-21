# Usage

`cred` with no arguments opens the project list. `cred --help` prints help.

## Password entry

Master passwords use an editable field, masked as `•` by default. Ctrl+R toggles
show/hide; the screen labels the current state as Hidden or Visible. Arrow keys,
Home/End, Backspace/Delete and paste work in the field. Tab / Shift+Tab moves
between the field, Open/Continue and Cancel. Enter advances from the field;
Enter on Open/Continue submits it. Empty input shows an error. Spaces are
preserved. Esc or Ctrl+C cancels.

Every entry, including password confirmation and retries, starts masked. The
password screen uses the terminal's alternate screen and closes after submission
or cancellation, without printing the password to command output.

Opening an existing vault or connecting to one allows three password attempts.
A wrong password clears the field and shows the remaining attempts. Empty input
does not use an attempt. Missing or damaged vaults stop immediately. Creating a
vault still requires entering the new password twice.

## Create a vault

```sh
cred init
```

With no profile argument, `cred init` opens a single setup flow: choose the vault
type, enter its details and password, review, then explicitly Create or Connect.
Use arrows and Enter for choices; Tab / Shift+Tab moves through fields and
buttons. Enter advances from a field and activates a focused button. Back returns
to earlier input steps, preserving normal fields and clearing passwords. Esc or
Ctrl+C cancels before creation. A terminal is required.

| Option | Prompts and behavior |
| --- | --- |
| Create a local vault | Profile name (starts as `local`), new master password twice |
| Connect to an existing team vault | Profile name, repository URL, existing master password (up to three attempts) |
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

After successful guided setup, the result screen shows the profile, vault path
and a usage example. Choose Finish, or explicitly choose Use this vault by default
to change the selected default. With `CRED_VAULT` set, explicitly select a profile
or unset the variable before relying on the default.

`cred init personal` (or `cred --vault personal init`) skips the menu and creates
a local profile using the same password, review and result screens, skipping
the known name. `cred connect team <repository>` remains the direct connection
shortcut. To publish an existing local
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

The service name is prompted when it is omitted. Choose `email`, `password`,
`username`, `url`, `api_key`, `token`, or **Custom field** using arrows or Tab,
then Enter. Custom field lets you enter any nonblank name. Next enter a masked value and choose Visible
or Secret. Choose **Finish** to save the list; Esc cancels without saving.
Text-only prompt adapters retain free-text names and finish with an empty name.
The visibility choice starts on Visible for a new field. An empty value is allowed.

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
The value is prompted in a masked field. The shared visibility selection starts
on the current flag. A missing field is an error.

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

Every delete shows a confirmation panel with Cancel selected. Choose Delete
explicitly and press Enter to proceed; Enter on Cancel or Esc leaves the file
unchanged.
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
project's service list. After unlocking, moving between screens
keeps the same unlocked snapshot.

Enter moves forward: project, then service, then field. Enter on a field
copies that value to the clipboard. Arrow keys, `j`, and `k` move. `/` filters
the current list. `s` shows the selected secret for 5 seconds, then masks it
again. Esc or left returns to the previous screen, including from a service
list opened with `cred <project>`. On the project list, Esc quits. `q` and
Ctrl+C quit from any screen. In forms, `q` is ordinary text; Esc cancels the
form and Ctrl+C exits.

### Create, edit and delete in the browser

The bottom bar shows `n new` and, when an item is selected, `e edit` and
`d delete`. These act on projects in the project list, services inside a
project, and fields inside a service. Empty projects and services can be created directly.

- `n` opens a new-item form. `e` edits the selected item.
- Projects and services have a name input. Fields have name, value and secret
  inputs. Editing pre-fills the existing values; secret values are masked, and
  new fields default to secret.
- Forms open in a framed panel. Tab / Shift+Tab moves through the inputs,
  Secret checkbox, Generate password (fields only), Save and Cancel buttons. Space toggles the checkbox when
  focused. Enter advances to the next item until a button is focused; Enter on
  Save saves, and Enter on Cancel closes the form. Esc cancels immediately
  without applying changes.
- In field forms, Ctrl+G or Enter on Generate password replaces the draft value
  with a 24-character password generated using `crypto/rand`, containing uppercase
  and lowercase ASCII letters, digits and symbols (`!@#$%^&*()-_=+`). It enables
  Secret and focuses the masked value. Ctrl+R shows/hides the focused secret value.
  Generate again to replace the draft; only Save writes it to the vault.
- On small terminals, the panel scrolls to keep the focused input or button
  visible. PgUp / PgDown scrolls the panel body to inspect longer messages.
- Renaming preserves all descendants. Invalid or duplicate names are rejected
  without overwriting existing records.
- `d` opens a confirmation naming the selected item and, for a project or service,
  counting the descendants that will also be deleted. Cancel is selected initially;
  use Left/Right or Tab to choose Delete, then Enter to confirm. Esc cancels.

Successful changes are saved immediately through the encrypted vault workflow,
and the browser returns to the same list. Errors keep the form and its inputs.
Shared vault saves use the existing Git sharing and conflict prompts; the browser
releases the terminal for those prompts and resumes afterward. If a failed shared
save retains recovery data, resolve it with `cred vault retry` or `cred vault
discard` before saving another change.

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

The [terminal interface guide](interface.md) describes shared screen layout,
keyboard behavior and plain-text output compatibility.

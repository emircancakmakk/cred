# Changelog

## Unreleased

### Added

- Vault-scoped saved email management in the TUI (`m`), with multiline bulk input, validation, duplicate review, editing and confirmed deletion.
- Searchable saved-address selection in new and edit email field forms via Ctrl+E or Choose saved email. Selection copies an editable value.
- Record-level Git merging and encrypted recovery for the email collection.

### Upgrade notes

- Existing v1 vaults remain usable. The first successful collection save upgrades that vault to v2; upgrade every shared-vault client first. Older clients reject v2.
- Removing or editing saved addresses never changes existing credentials. `cred add`, `cred set` and the v1 credential import format retain their existing scope.

## v0.5.0 — 2026-10-21

### Added

- Shared email, password, username, url, api_key and token presets, plus custom field names, in CLI and TUI field creation.
- Secure 24-character password generation in new and edit field forms through Ctrl+G or the Generate password button. Generated values are masked and marked Secret until explicitly saved.

### Fixed

- Enter advances from the Secret checkbox directly to Save without replacing manually entered values with a generated password.
- Updated field editor tests to follow the preset selection step and verified the combined flow through a real terminal.

### Upgrade notes

- Vault encryption, stored data format and existing passwords remain compatible.
- In `cred add`, choose Finish to save all added fields; Esc cancels without saving.
- Use Tab to focus Generate password or Ctrl+G to generate. Enter follows the normal input-to-save path.

## v0.4.0 — 2026-10-21

### Added

- Create, rename, edit and delete projects, services and fields directly in the TUI, with persistent encrypted saves and explicit deletion confirmation.
- Shared terminal interface components for forms, selections, confirmations, keyboard help and responsive panels.
- Editable masked master-password entry with Ctrl+R show/hide and up to three unlock attempts.
- A guided setup flow with Back navigation, review before creation, progress and a result screen with optional default-vault selection.

### Changed

- Setup, unlocking, the browser, CRUD forms, CLI prompts, Git conflict choices and recovery confirmations use a common terminal design and keyboard conventions.
- Enter advances through form fields; actions run when their button is focused. Tab / Shift+Tab moves between fields and buttons.
- Human-facing command output uses shared styling on terminals while pipes, files and raw `--show` output retain their plain-text behavior.
- Narrow terminals support wrapped help, scrolling panels and bounded Unicode input and browser rows.

### Fixed

- Removed repeated master-password headings and descriptions from the unlock and connection retry screens.
- Excluded `.vibe-wise` workspace files from Git tracking.

### Upgrade notes

- Vault encryption and plaintext format remain unchanged; existing vaults and passwords continue to work.
- Use Tab or Enter to reach Open/Continue in password forms, then press Enter to submit. Ctrl+R shows or hides the focused sensitive field.

## v0.3.0 — 2026-10-18

### Added

- Named local and Git vault profiles, with per-command selection and a configurable default.
- Shared encrypted vaults backed by a private Git repository and a persistent local checkout.
- Automatic three-way merges for independent field edits and interactive choices for conflicting changes.
- Interactive setup for local vaults, existing team vaults and new shared vaults in empty repositories.
- `cred status` and `cred status --remote` for password-free local status and optional remote checks.
- `cred vault retry <name>` to recover an unpublished change or complete the first shared-vault push.
- `cred vault discard <name>` to abandon changes after confirmation and validation of the remote vault, or cancel unfinished setup against an empty remote.
- Sanitized Git error messages, actionable recovery hints and terminal progress indicators.
- `cred --version` to identify the installed release.

### Changed

- The default local vault now lives at `cred/vaults/local/vault.age` under the user config directory; other local profiles use `cred/vaults/<name>/vault.age`.
- Parameterless `cred init` opens the setup menu, including when `CRED_VAULT` is set. Use `cred init <name>` for direct local profile creation.
- `connect`, `vault` and `status` are reserved command names.

### Upgrade notes

```sh
brew update
brew upgrade emircancakmakk/tap/cred
cred --version
```

To keep using a vault at the previous macOS path:

```sh
export CRED_VAULT="$HOME/Library/Application Support/cred/vault.age"
cred
```

On Linux, point `CRED_VAULT` at the existing `$XDG_CONFIG_HOME/cred/vault.age`
or `~/.config/cred/vault.age`. Alternatively, copy the old file to the new
`cred/vaults/local/vault.age` location under the user config directory without
overwriting an existing vault. No `cred init` is needed to open an existing vault.

- Existing vault encryption and plaintext format remain version 1. Existing passwords still work.
- Old vault paths are not migrated automatically. Set `CRED_VAULT` to an existing file to continue using it, or copy it to the new local path without replacing another vault. Do not initialize over existing data.
- Git profiles require Git authentication and author identity to be configured. Remote repositories are created by the user, not cred.
- Git vault members share the vault password. Offline Git vault operations, per-project authorization and master-password rotation are not included.

## v0.2.0

- JSON import with validation, duplicate skipping and secret flags.

## v0.1.0

- Initial local encrypted vault, terminal selector, field management and clipboard support.

# Changelog

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

- Existing vault encryption and plaintext format remain version 1. Existing passwords still work.
- Old vault paths are not migrated automatically. Set `CRED_VAULT` to an existing file to continue using it, or copy it to the new local path without replacing another vault. Do not initialize over existing data.
- Git profiles require Git authentication and author identity to be configured. Remote repositories are created by the user, not cred.
- Git vault members share the vault password. Offline Git vault operations, per-project authorization and master-password rotation are not included.

## v0.2.0

- JSON import with validation, duplicate skipping and secret flags.

## v0.1.0

- Initial local encrypted vault, terminal selector, field management and clipboard support.

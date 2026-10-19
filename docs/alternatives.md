# How cred compares

Choose cred when you want structured credentials in the terminal, stored as one
fully encrypted vault, with concurrent writes reconciled by the application.
Git transports ciphertext; cred decrypts the original, local and remote snapshots
in memory to merge fields, then encrypts the result before publishing it.

| Tool | Primary workflow | Storage and sharing | Where cred differs |
| --- | --- | --- | --- |
| [pass](https://www.passwordstore.org/) | Unix password store with files and extensions | Individual GPG-encrypted files; optional Git repository, multiple recipient keys and folder-specific recipients | cred stores project/service/field records in one age passphrase-encrypted vault and reconciles concurrent field edits inside it. |
| [gopass](https://www.gopass.pw/) | Developer password manager with team sharing and integrations | Local encrypted stores, Git versioning and synchronization | cred focuses on a small structured terminal workflow and explicit field-level three-way merging within a fully encrypted vault. |
| [1Password CLI](https://developer.1password.com/docs/cli/) | Access 1Password items, manage permissions and supply secrets to applications | 1Password accounts and vaults, with service-account and application integrations | cred uses your Git repository and a shared vault password, without a 1Password account; it has no equivalent user/group permission system or runtime secret injection. |
| [SOPS](https://getsops.io/) | Encrypt configuration files for infrastructure and application workflows | Structured formats can keep keys visible while encrypting values; supports age, PGP and cloud KMS identities, plus whole-file binary encryption | cred is an interactive credential browser; it encrypts the whole vault including names and resolves concurrent record edits in the client. |

Git support is not unique to cred, and this comparison does not claim that other
tools cannot resolve conflicts. The defining combination here is a fully encrypted,
structured vault plus cred's built-in field-level reconciliation during publication.

## What merging means

- Different fields merge automatically, including fields in the same service.
- A field's value and secret flag are merged together.
- Incompatible edits to the same field, or deletion versus edits beneath it,
  ask for team, mine or cancel. Cancel is the default; values are not shown.
- Case-only name collisions stop the merge. Failed publication retains encrypted
  recovery snapshots for [retry or discard](recovery.md).

## Tradeoffs

Members share one master password and can read the entire vault. There are no
per-project permissions or master-password rotation command. Removing Git access
cannot revoke downloaded vaults; Git history retains previous encrypted versions.
Git profiles require repository access for reads and writes. Use a local profile
for offline work; it does not automatically synchronize with a Git profile.

Use the same conflict-aware client for all writers; manual or force replacement
of the encrypted vault bypasses these protections. See the [Git guide](git-profiles.md)
and [security model](security.md).

The linked official documentation describes each alternative; cred's behavior is
implemented in this repository. This is a workflow comparison, not a security ranking.

[← README](../README.md) · [Documentation](README.md)

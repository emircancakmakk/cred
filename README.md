<p align="center">
  <img src="assets/cred-icon.png" alt="" width="96" height="96" />
</p>

<h1 align="center">cred</h1>

<p align="center">
  An encrypted credential vault for the terminal, shared through Git with field-level, conflict-aware merging.
</p>

<p align="center">
  <a href="https://github.com/emircancakmakk/cred/releases/latest"><img src="https://img.shields.io/github/v/release/emircancakmakk/cred?label=release&color=2eb857" alt="Latest release" /></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-000000" alt="macOS and Linux" />
  <img src="https://img.shields.io/badge/license-MIT-2eb857" alt="MIT license" />
</p>

![Create a vault, add a credential and browse it in the terminal](assets/quick-start.gif)

## Why cred?

- **Share through your own Git repository.** Keep one encrypted vault in a private repo, using your existing Git authentication.
- **Merge independent field edits automatically.** Two people can update different fields, even in the same service; cred merges their changes before publishing.
- **Resolve conflicts deliberately.** Edits to the same field prompt for team, mine or cancel, without displaying the values.
- **Manage credentials in the TUI.** Create, edit and delete projects, services and fields with `n`, `e` and `d`.
- **Find credentials from the terminal.** Browse project → service → field, filter names and copy a value. Local vaults also work offline.

cred combines a fully encrypted vault with application-level, three-way merging of
its fields. Git sharing is also available in pass and gopass; cred focuses on this
structured credential workflow. For differences from those tools, 1Password CLI
and SOPS, see [the comparison](docs/alternatives.md).

## Install

```sh
brew install emircancakmakk/tap/cred
```

Prefer a source build? See [build instructions](docs/README.md).

## 30-second quick start

```sh
cred init
cred
```

1. In `cred init`, choose **Create a local vault**, accept the name `local` and enter a master password of at least 12 characters twice. Review and choose Create, then optionally choose Use this vault by default.
2. Run `cred` and unlock. Press `n` to create a project, Enter to open it, then `n` to create a service. Open the service and press `n` to add a field; Tab moves between inputs and the Save/Cancel buttons. Enter advances through inputs; press Enter on Save to save.
3. Browse project → service → field. Enter on a field copies its value; `e` edits, `d` asks to delete, and `q` quits.

In new and edit field forms, choose **Generate password** or press Ctrl+G to
fill a hidden 24-character password with uppercase and lowercase letters,
digits and symbols. Ctrl+R shows/hides the focused value. Choose Save to persist
it; Esc cancels the draft.

Save reusable email addresses with `m` in the browser. Press `n` to paste a
batch, review it, then Save. In an `email` field form, choose **Choose saved
email** or press Ctrl+E; selection copies the address into the editable draft.
Addresses belong to the encrypted vault and are shared in Git vaults.
See [saved email setup and reuse](docs/usage.md#saved-email-addresses), including
format upgrade notes for shared vaults.

Clipboard copying currently uses macOS `pbcopy` / `pbpaste`. On Linux, retrieve a
value with `cred my-project vercel password --show`.

Want a shared vault? Follow the [Git setup guide](docs/git-profiles.md).

## Documentation

[Commands and selector](docs/usage.md) ·
[Git profiles and merging](docs/git-profiles.md) ·
[Retry / discard](docs/recovery.md) ·
[Configuration and status](docs/configuration.md) ·
[Name matching](docs/name-matching.md) ·
[Security and limits](docs/security.md)

[All documentation](docs/README.md) · [Changelog and upgrade notes](CHANGELOG.md)

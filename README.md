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
cred add my-project vercel
cred
```

1. In `cred init`, choose **Create a local vault**, accept the name `local` and enter a master password of at least 12 characters twice. Answer `y` to use it by default.
2. In `cred add`, unlock the vault, enter a field name such as `password`, its hidden value, and `y` for secret. Submit an empty field name to finish.
3. Run `cred`, unlock, then press Enter to browse project → service → field. Enter on a field copies its value; `q` quits.

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

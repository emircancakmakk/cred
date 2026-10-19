# Documentation

Start with the [30-second quick start](../README.md#30-second-quick-start).

- [Usage](usage.md): guided setup, add, import, edit, list, delete, selector and clipboard.
- [Local and shared Git profiles](git-profiles.md): connect a team, share a vault and merge concurrent edits.
- [Retry and discard](recovery.md): recover unpublished changes and incomplete setup.
- [Configuration and status](configuration.md): vault paths, environment overrides and diagnostics.
- [Name matching](name-matching.md): exact names, case folding, prefixes and validation.
- [Security and known limits](security.md): encryption, storage, clipboard and shared access.
- [Alternatives](alternatives.md): how cred compares with pass, gopass, 1Password CLI and SOPS.
- [Recording the demos](../demos/README.md): reproduce the terminal GIFs with VHS.

For source builds, install Go 1.27.2 or newer (see `go.mod`), then run:

```sh
make build
make test
```

`make build` writes `./cred`. Put it on your `PATH`, or use `go run .` from the repository.

[← README](../README.md) · [Changelog and upgrades](../CHANGELOG.md)

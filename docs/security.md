# Security and known limits

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


[← README](../README.md) · [Documentation](README.md)

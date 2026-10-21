# Local and shared Git profiles

![Concurrent field edits and conflict resolution](../assets/git-merge.gif)

Git must be installed and authenticated (for example with an SSH key). Configure
`git config --global user.name` and `user.email` before writing to a Git vault.
Use a private repository and grant access individually through GitHub/GitLab.

```sh
# Guided setup: local, connect to a team, or create a new shared vault.
cred init

# Direct local setup (no menu).
cred init local

# Separate personal vault.
cred init personal
cred vault use personal

# First team member: create an empty private repository, then run cred init
# and choose "Create a shared vault in an empty Git repository".
# Enter the profile name, repository URL and a new master password.
# No intermediate local seed profile is needed.

# Other members: connect to the existing encrypted vault; no init required.
cred connect team git@github.com:company/team-vault.git

cred --vault team add my-project vercel
cred --vault team my-project vercel password
cred vault list
cred vault use team
cred vault use local  # select the local profile
```

`connect` asks for the existing vault's master password and validates it before
saving the connection. `--from` publishes only into an empty repository and
leaves the source local vault independent. New connections do not change the
default. Git profiles follow the remote's default branch. Each vault has its own
password. Machine-local settings live in `cred/profiles.json` in the user config
directory; passwords are never stored there.

`--vault NAME` selects a profile for one command. Without it, `CRED_VAULT` takes
precedence over the selected default and always means a local file. Explicit
`--vault local` selects the profile named `local`, independently of the environment
override. Names `connect`, `vault` and `status` are reserved commands. `local` is an ordinary
profile name; the local setup screen suggests `local`, while `cred init personal`
creates `personal` directly. There is no legacy vault location or automatic migration.
Parameterless `cred init` always opens the setup menu, even when `CRED_VAULT` is
set; the wizard creates named profiles rather than a file at the override path.

Local profiles never access Git. Local vaults live at `cred/vaults/<name>/vault.age`
inside the user config directory. Git profiles clone once into the private managed
checkout `cred/vaults/<name>/repo/` and reuse it. Each operation fetches and updates
to the remote branch before opening `repo/vault.age`; Git never merges the
encrypted file. Network access is required, including for reads. Uncommitted edits
or unpublished commits in the managed checkout stop normal operations; retained
changes are recovered with `cred vault retry`. Recovery may reset the managed
checkout after reading the encrypted recovery pair. Do not edit the managed repo
manually. Recovery files stay outside `repo/`, in the profile directory.
Only `vault.age` is staged by cred. Values are
decrypted only in memory, and commit messages contain no record names or values.
The active profile is displayed in the selector and on named/Git operations.

Concurrent writes use ordinary non-force pushes. On rejection, cred fetches the
latest version and compares the original, local and remote vaults. Different
fields merge automatically. A field's value and secret flag are one unit. Same
field conflicts and deletion versus descendant edits prompt for team, mine, or
cancel; the default is cancel and values are not displayed. Case-only name
collisions stop the merge. At most three publication attempts are made.
Local processes are locked for the entire read/modify/write session.

Saved email addresses also belong to the encrypted vault and are shared with the
team. Different records merge automatically; conflicting edits or edit/delete
changes to one record use Team / Mine / Cancel with an opaque record ID in place
of the address. Duplicate concurrent additions converge to one address. Updating
or deleting a saved address never changes credential fields that copied it.
Upgrade all clients before the first collection save, which upgrades the vault
to v2. Older clients reject v2. See [email storage and upgrades](usage.md#storage-sharing-and-upgrades).

If publication fails or a merge is cancelled, cred retains encrypted change and
base files outside the repository and reports an error instead of success.
Further Git operations on that profile stop until you retry or discard:

```sh
cred vault retry team     # merge and publish the retained change
cred vault discard team   # abandon it and restore a validated remote snapshot
```

Retry merges the retained change with the current remote and asks again for any
conflicts. Only a concurrent remote update triggers automatic merge/retry. Other
Git failures stop immediately and retain the encrypted change, with a hint for
missing Git, connectivity/DNS, authentication/access, missing author identity,
timeout or server policy rejection. Unrecognized failures are identified as
unknown; raw Git diagnostics and credential-bearing repository URLs are not
printed. Git hooks are disabled in managed working copies. No offline edits or automatic fallback to local storage occur.

Git history retains old encrypted vaults. Removing a member's repository access
cannot revoke downloaded copies. Rotate the underlying service credentials when
necessary. This version has no master password rotation command or per-project
permissions; every member with repository access and the password can read the
whole vault. Require the same conflict-aware client for all writers and avoid
manual/force replacement of `vault.age`.

For failed publication and incomplete setup, see [retry and discard](recovery.md).


[← README](../README.md) · [Documentation](README.md)

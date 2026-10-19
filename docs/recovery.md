# Retry and discard

If a Git change cannot be published, cred retains encrypted change and base snapshots.
Normal operations stop until recovery succeeds or you discard the change.

```sh
cred vault retry team
cred vault discard team
```

Retry merges the retained change with the current remote and prompts again for conflicts.
It can also complete an unfinished first push. A different remote master password
stops retry and preserves the snapshots. See [shared setup](usage.md#create-a-vault).

## Discard pending Git changes

```sh
cred vault discard team
```

Discard requires a Git profile and asks for confirmation, defaulting to no. There
is no flag to skip confirmation. Cancellation does not access the network or
change vault data. If there is no pending change, leftover recovery file or
incomplete setup, the command reports `nothing to discard` without asking for a
password or accessing the network.

After confirmation, cred checks the remote and fetches the selected branch. It
asks once for the **remote vault's** master password and validates the exact
fetched snapshot in memory before resetting the managed checkout. Wrong passwords,
corrupt or missing remote vaults, symbolic links, missing branches and connection
failures leave pending data intact. There is no offline fallback. The validated
commit is pinned, so a remote update during verification cannot replace it with
an unvalidated snapshot. No changes are pushed to the remote repository.

After restoring that commit, cred removes the pending change, its base and their
backups. It also removes the checkout's untracked `vault.age.bak`, preserving
tracked remote backups and unrelated untracked files. File cleanup errors are
reported instead of success; run the same discard command again to finish after
resolving the reported filesystem problem. The primary pending file is removed
last, and discard does not need the recovery base to be readable or present.

For an incomplete first setup, confirmation explicitly explains that a still-empty
remote means removing the unfinished **local** profile, checkout and recovery
files. This cancellation does not ask for a master password or modify the remote.
If a remote vault has since appeared, cred validates it with its current password
and restores it, keeping the profile and completing setup instead. A failed config
save rolls the moved local directory back. If final directory cleanup fails, the
error identifies where remaining data is stored. The selected default is preserved.


[← README](../README.md) · [Documentation](README.md)

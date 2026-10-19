# Record the terminal demos

Install [VHS](https://github.com/charmbracelet/vhs#installation) and its dependencies:

```sh
brew install vhs ffmpeg ttyd
```

Go (the version in `go.mod` or newer), Git, Python 3, Bash and the Menlo font are
also required. On Linux, change `FontFamily` in both tapes to an installed
monospace font. VHS may download its headless browser on the first run.

From the repository root:

```sh
make demo
```

This builds the current source into a temporary directory and records:

- `assets/quick-start.gif`: guided local setup, one secret field, then browsing
  project → service → field. It exits before copying, leaving the clipboard alone.
- `assets/git-merge.gif`: two real Git profiles edit a shared vault concurrently.
  Different fields merge automatically; the next scene edits the same field and
  chooses `2` (mine) at cred's conflict prompt.

The runner gives its child processes disposable home/config directories, clears
`CRED_VAULT` and isolates Git identity/configuration. It creates a local bare Git
repository; it does not connect to GitHub or use existing vaults. Temporary data
is removed when the runner exits. GIFs are written to `assets/` and are not uploaded.
All passwords and values in the tapes/helpers are public, fake demo data.

The Git helper uses Python's standard-library pseudo-terminals to coordinate
actual CLI prompts. Alice pauses after unlocking while Bob publishes, ensuring
the demo exercises concurrent publication. The helper verifies both independent
edits survive and that the selected conflict value is published. Comments and
verification lines explain the scenario; command output comes from cred itself.
The master password is supplied through terminal input, not an environment variable.

To run the Git scenarios and assertions without rendering, use
`python3 demos/record.py --check`. To re-record only one GIF, use
`python3 demos/record.py quick-start` or `python3 demos/record.py git-merge`.

Edit `.tape` files for presentation/timing. `Wait+Line` synchronizes prompts;
fixed pauses provide reading time. Recordings use 1000×600 pixels at 12 fps with
a dark theme. Aim for less than 5 MB per GIF and inspect representative frames
after changing tapes or CLI output.

[← Documentation](../docs/README.md)

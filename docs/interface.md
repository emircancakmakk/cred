# Terminal interface

cred uses one terminal design system for setup, unlocking, browsing and command
prompts. The shared components live in `internal/terminalui`; vault encryption,
Git operations and persistence stay outside that package.

## Layout and theme

Each interactive screen has the same structure: a `cred` header with context,
a rounded content panel, a status area and keyboard help. Fields have labels
and a visible `>` focus marker. Form, choice and confirmation panels are capped
at 64 columns; the browser is capped at 96 columns. Panels are horizontally
centered, with left-aligned content.

The theme adapts to light and dark terminals. Accent identifies focus and
headings; normal and muted ink separate content from supporting text. Success,
warning and danger styles indicate outcomes. Focus, errors and destructive
actions always include text or markers, so color is not required to use cred.
The terminal's font is retained.

Long labels and descriptions wrap. Long input values scroll horizontally
without changing the stored value. Small windows scroll the panel body to keep
the focused item visible; PgUp/PgDown inspects content outside the viewport.
The header and help stay visible. Very small windows ask to resize and still
allow cancellation.

## Keyboard contract

| Surface | Behavior |
| --- | --- |
| Form | Tab / Shift+Tab moves through fields and buttons. Enter advances from a field and activates a focused button. |
| Sensitive field | Starts masked; Ctrl+R shows/hides the focused field. |
| Selection | Arrows or Tab move; Enter chooses. |
| Confirmation | Cancel is initially selected; explicitly choose the action to proceed. |
| Navigation | Arrows / j / k move, Enter opens or copies, Esc / left goes back, `/` filters. |
| Cancellation | Esc cancels a form; Ctrl+C exits the interaction. |
| Setup | Back returns to an earlier input step. Normal input is retained; passwords are cleared and masked. |

During a setup write or Git operation, Ctrl+C requests exit after the current
operation reaches its cleanup boundary. Finishing work is not reported as if it
had been rolled back. Once creation succeeds, the result screen cannot return
to the input steps.

## Screen inventory

| Area | Shared presentation |
| --- | --- |
| `cred init` | Choice → details → password → review → progress → result, in one alternate-screen session. |
| `cred init <name>`, `cred connect` | Same setup components, skipping details supplied as arguments. |
| Unlock / password confirmation | Sensitive field, visibility label, Open/Continue and Cancel actions. |
| Project / service / field browser | Single-list navigation in the shared frame, with context and filtering help. |
| TUI create / edit | Labeled fields, Secret choice, Save and Cancel. |
| TUI delete / `cred rm` | Destructive confirmation with affected-record information and Cancel selected. |
| `cred add`, `cred set` | Shared input and visibility selection. Empty field values remain valid. |
| Git conflicts | Explicit Cancel / Team / Mine choice. |
| Recovery / discard | Shared password and destructive confirmation components. |
| `ls`, `status`, profile list, help | Normal-screen output with shared typography and semantic colors on a terminal. |
| Progress and operation messages | Shared accent, success, warning and error styles. |

Interactive secrets stay on the alternate screen. Submission or cancellation
clears the input view before returning to normal output. Explicit reveal can
still be captured by terminal logging or screen recording.

## Output compatibility

stdout and stderr are detected independently. Pipes and files retain the existing
plain text, without panels or ANSI styling. `NO_COLOR` and `TERM=dumb` disable
styling of normal command output. `--show` always writes only the requested value
and its trailing newline, including on a terminal.

Command names, flags, vault formats and Git recovery semantics are unchanged.
Injected `Prompter` implementations retain their existing method contract;
production prompts use typed form and selection requests through an optional
adapter instead of interpreting question text.

## Extending the interface

Use the shared frame, inputs, actions, selection and semantic colors for a new
screen. Supply explicit context, labels and help. Keep business decisions in
`cmd` or the relevant domain package. Validate at the action boundary, preserve
inputs on errors, and clear sensitive inputs when leaving a step.

Check representative views at 80×24, 40×20 and 120×30, with long Unicode values
and without color. `go test ./...` covers form focus, safe defaults, setup
transitions, output compatibility and a real PTY setup/unlock/CRUD flow when
Python 3 and Unix terminal support are available.

# Project Map

## Purpose
Encrypted credential vault for the terminal, optionally shared through Git.

## Requirements and chosen design
- Contextual CRUD at all three levels: project list manages projects, project contents manage services, service contents manage fields.
- Learner chose bottom shortcuts n (new), e (edit selected), d (delete after confirmation); u was corrected to e.
- Learner delegated form interaction design and approved implementation with Uygula.
- Approved AI additions: list replaced by form preserving breadcrumb; project/service name, field name/value/secret; Tab/Shift+Tab focus, Space checkbox, Enter save, Esc cancel; existing values prefilled and secrets masked; new fields secret by default.
- Approved delete flow names the target, counts descendants and defaults to Cancel. Empty projects/services can be created; renames preserve descendants; duplicates cannot overwrite records; save errors preserve form inputs.

## Components
- go.mod: Go, Cobra, Bubble Tea/Bubbles/Lip Gloss, age.
- cmd/tui.go: unlock and snapshot conversion; launches tui.RunEditable.
- internal/tui/tui.go: hierarchy browsing, filtering, copying, temporary reveal; routes contextual CRUD.
- internal/tui/edit.go: forms, deletion confirmation, typed Change/Commit bridge, save results and list restoration.
- cmd/tui_edit.go: clones last persisted vault, applies exact mutation, invokes existing a.save; refreshes actual published Git data and merge base after success; protects retained recovery from subsequent edits.
- internal/vault/edit.go: empty project/service creation and descendant-preserving renames with existing naming/case rules.
- internal/vault/store.go: encrypted persistence.
- cmd/profiles.go: existing Git publishing, merge conflicts and encrypted recovery.
- README.md and docs/usage.md: TUI CRUD quick start and usage.

## Main Flow
Implemented: runTUI → unlock → projectsOf → RunEditable → browse/create/edit/delete → Commit(Change) → clone/validate/mutate → a.save → encrypted local save or Git share/merge → persisted snapshot → same parent list.
Failed validation/persistence preserves the last displayed successful snapshot and form inputs.

## Data and Trust Boundaries
TUI holds decrypted values; master password stays in cmd closure and is wiped when runTUI returns. Secret inputs are masked. Name/flag-only field edits preserve imported values byte for byte even when single-line widget sanitizes display. Git conflict prompts use the existing terminal prompt; Bubble Tea releases input/terminal ownership during save and resumes afterward. Failed shared saves retaining recovery require existing retry/discard flow before another save.

## Build and Deployment
Verified: go test ./... passed. GORACE=atexit_sleep_ms=0 go test -race ./... passed. Default race exit sleep causes existing TestClearSurvivesParentExit to observe an already-cleared clipboard; no clipboard code was changed. git diff --check and gofmt checks passed.
No deployment performed.

## Unknowns
No pending decisions for approved CRUD scope. Imported multiline values are preserved unchanged when editing name/secret flag; the form uses a single-line value input.

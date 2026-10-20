# Learning Progress

## Learning scope and preferences
- User reports intermediate programming experience and beginner familiarity with Go/Bubble Tea.
- User knows the project structure and wants UX updates only, without Go language or syntax lessons.
- User chose default learning preferences: meaningful decisions discussed first; AI writes agreed code.

## TUI CRUD requirements and controls
- Learner wants contextual CRUD directly while navigating project, service and field lists.
- Learner chose bottom shortcuts n for a new item of the current level, e to edit selection, and d to delete after confirmation, consistently across levels.
- Learner clarified that u was a mistaken expression; no separate u update action.
- These are learner product and interaction decisions, not evidence of Go proficiency.

## Form design and implementation
- Learner delegated create/edit interaction design to AI.
- Learner approved the presented implementation scope with Uygula: temporary forms, name/value/secret controls, Tab/Shift+Tab focus, Space checkbox, Enter save, Esc cancel; masked secrets and default secret for new fields; descendant counts and default Cancel in deletion; immediate encrypted persistence with error/input retention; empty project/service creation and descendant-preserving renames.
- Implemented in internal/tui/edit.go, cmd/tui_edit.go, internal/vault/edit.go and existing TUI entry points; usage and quick-start documentation updated.
- Tests cover keyboard scope, masks, cancel/default-delete, filtered selection, context restoration, failures preserving inputs, real Bubble Tea save lifecycle, exact imported-value preservation, validation and duplicate protection, encrypted CRUD round trips, Git merge refresh/repeated saves and protection of retained recovery.
- Verified: go test ./... passed; GORACE=atexit_sleep_ms=0 go test -race ./... passed. Default race run failed existing TestClearSurvivesParentExit due race runtime exit sleep exceeding clipboard TTL; adjusted runtime exit delay only, no clipboard changes.
- git diff --check and gofmt checks passed.
- No further decision pending for this scope; no claims of demonstrated engineering reasoning beyond the learner's interaction choices.

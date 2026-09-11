# Task List — Go 1.27 + UX Overhaul

## Phase 1 — Toolchain
- [x] **Task 0** — Go 1.27.0 in go.mod, Dockerfile, ci.yml, release.yml

## Phase 2 — Navigation and discovery
- [x] **Task 1** — overlay.go: centred modal helper
- [x] **Task 2** — help_overlay.go: `?` / `F1` scrollable keybinding reference
- [x] **Task 3** — filter.go: `/` live filter for all list tables
- [x] **Task 4** — column sorting with `<` / `>` and header markers

## Phase 3 — Feedback
- [x] **Task 5** — toast.go: transient success/error/info notifications
- [x] **Task 6** — contextual loading text per action
- [x] **Task 7** — `ctrl+r` auto-refresh toggle

## Phase 4 — Data richness
- [x] **Task 8** — TopicInfo gains Messages and Size, fetched lazily
- [x] **Task 9** — consumer-group per-partition lag drill-down

## Phase 5 — Quick actions
- [x] **Task 10** — `y` copy selection to clipboard
- [x] **Task 11** — `g` / `G` jump, broker detail pane

## Phase 6 — Documentation
- [x] **Task 12** — CHANGELOG.md, README keybinding table

## Verification
- [x] `go build ./...`
- [x] `go vet ./...`
- [x] `go test ./...`
- [x] `make lint`

# Implementation Plan: Go 1.27 + UX Overhaul

## Overview

Two pieces of work:

1. Bump the Go toolchain to 1.27.0 across `go.mod`, `Dockerfile`, and both GitHub
   Actions workflows.
2. A broad user-experience overhaul of the TUI, covering navigation, feedback,
   data richness, and quick actions.

Design inspiration is taken from `axonops/cqlai`, specifically:

- A centred modal overlay drawn on top of the current view rather than replacing
  it (help window).
- A status bar built from a list of segments that are fitted to the terminal
  width, so the bar never wraps to a second row.
- One description of the layout shared by drawing and by scroll/hit handling.

cqlai is on bubbletea v2 / lipgloss v2 (`charm.land/...`); kconduit stays on v1.
Only the patterns are borrowed, not the dependency versions. Full-screen centring
uses `lipgloss.Place`, which is ANSI-safe, instead of cqlai's byte-slicing
overlay.

## Work Breakdown

### Phase 1 — Toolchain

- **Task 0** — Go 1.27.0 in `go.mod`, `Dockerfile`, `.github/workflows/ci.yml`,
  `.github/workflows/release.yml`.

### Phase 2 — Navigation and discovery

- **Task 1** — `pkg/ui/overlay.go`: `renderOverlay(background, box, w, h)` helper
  that centres a bordered box over a dimmed background.
- **Task 2** — `pkg/ui/help_overlay.go`: a scrollable, categorised keybinding
  reference opened with `?` (and `F1`), closed with `esc`. Rows are declared once
  in a table and reused by both the overlay and the footer help bar.
- **Task 3** — `pkg/ui/filter.go`: a `/` live filter applied to the topics,
  consumer-group, ACL, and broker tables. Case-insensitive substring match,
  `esc` clears, `enter` commits and returns focus to the table. The active
  filter and the match count are shown in the status bar.
- **Task 4** — Column sorting: `<` and `>` step through the orderings. Each
  column occupies two places, ascending then descending, so two keys reach every
  ordering without a third for direction. The active column's header carries a
  `▲`/`▼` marker.

### Phase 3 — Feedback

- **Task 5** — `pkg/ui/toast.go`: a transient one-line notification with success,
  error, and info severities, auto-expiring after 4 seconds via `tea.Tick`.
  Wired into topic create/delete, config edit, and ACL create/edit/delete.
- **Task 6** — Contextual loading text. The spinner line currently always reads
  "Connecting to Kafka cluster..."; it becomes a per-action label ("Loading
  topics…", "Refreshing consumer groups…").
- **Task 7** — Auto-refresh. `ctrl+r` toggles a 5-second refresh of the active
  tab; the status bar shows `⟳ auto` while it is on.

### Phase 4 — Data richness

- **Task 8** — `TopicInfo` gains `Messages` and `Size`, populated from partition
  high/low water marks and `DescribeLogDirs`. These are fetched lazily on a
  separate command so the topic list still paints immediately.
- **Task 9** — Consumer-group drill-down: `enter` on a group row opens a
  per-partition lag view (topic, partition, current offset, log-end offset, lag,
  member).

### Phase 5 — Quick actions

- **Task 10** — `y` copies the selected row, or the selected config value, to the
  system clipboard via `atotto/clipboard` (already an indirect dependency;
  promoted to direct).
- **Task 11** — `g` / `G` jump to the first and last row of the focused table;
  `enter` on a broker row opens a broker detail pane.

### Phase 6 — Documentation

- **Task 12** — Add `CHANGELOG.md` (Keep a Changelog + SemVer), and update the
  README key-binding table to match the new bindings.

## Keybinding Summary (after this work)

| Key | Action |
|-----|--------|
| `1`–`4` | Switch tab |
| `tab` / `shift+tab` | Cycle tab or panel |
| `/` | Filter current table |
| `<` / `>` | Step the sort order |
| `g` / `G` | First / last row |
| `y` | Copy selection to clipboard |
| `r` | Refresh |
| `ctrl+r` | Toggle auto-refresh |
| `enter` | Consume topic / drill into group / broker detail |
| `p` | Produce |
| `C` | Create topic or ACL |
| `e` | Edit config or ACL |
| `d` | Delete topic or ACL |
| `s` | Sessions |
| `a` | AI assistant |
| `?` / `F1` | Help overlay |
| `q` | Quit |

## Verification

`go build ./...`, `go vet ./...`, `go test ./...`, and `make lint` after each
phase. Manual smoke test of each new binding against a live cluster.

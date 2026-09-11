# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Keyboard reference overlay, opened with `?` or `F1` and closed with `Esc`. It
  scrolls, and it is drawn over the current view rather than replacing the
  screen.
- Live filtering on every list table with `/`. The filter narrows the rows as
  you type, `Enter` keeps it and returns focus to the table, and `Esc` clears
  it. The status bar shows the active filter and how many rows match.
- Column sorting with `<` and `>`. Each column occupies two places, ascending
  then descending, so the two keys reach every ordering. The sorted column's
  header carries a `▲` or `▼`. Numeric columns sort by value, so lag `100` no
  longer sorts before lag `9`.
- Message count and on-disk size per topic, shown as two new columns. They are
  measured after the topic list has painted, so the list is never held up by
  them.
- Per-partition lag breakdown for a consumer group, opened with `Enter` on the
  Consumer Groups tab: committed offset, log end offset, lag, and the member
  owning each partition. It filters, sorts, and totals the lag of whatever is
  currently shown.
- Broker detail pane, opened with `Enter` on the Brokers tab, showing the full
  address, role, rack, API version, listener count, and log-directory count that
  the table has to abbreviate.
- Clipboard support: `y` copies the selected row as tab-separated fields, or the
  selected value when the topic configuration panel is focused.
- Auto-refresh, toggled with `Ctrl+R`, reloading the active tab every five
  seconds. The status bar shows `⟳ auto` while it is on.
- `g` and `G` jump to the first and last row of the focused table.

### Security

- Control characters are stripped from every value the cluster supplies —
  topic names, consumer group ids, ACL principals, and a consumer's `client.id`
  — before it reaches a table cell or the clipboard. A consumer can set its
  `client.id` to anything, including terminal escape sequences; unfiltered,
  those could rewrite the screen, and an OSC 52 sequence copied with `y` could
  overwrite the operator's clipboard.

### Changed

- Go toolchain raised to 1.27.0, in `go.mod`, the Dockerfile, and both GitHub
  Actions workflows.
- Confirmations for create, edit, and delete now appear as a transient
  notification over the list view instead of being printed above the TUI or
  disappearing with the dialog that produced them.
- The loading line names the action in progress ("Loading ACLs…", "Measuring
  consumer group lag…") rather than always reading "Connecting to Kafka
  cluster...".
- The footer help is built for the active tab and trimmed to the terminal width,
  keeping `?` whichever hints have to be dropped. The status bar is pinned to a
  single row, so a long help line can no longer push the layout past the bottom
  of the terminal.
- The topic name column grows into whatever width the table has, instead of
  being truncated at a fixed 30 characters.
- The ACL table is now resized with the window; previously it was created after
  the resize that sized the other tables and kept its initial dimensions.
- Short-lived Kafka clients are given a copy of the connection configuration
  rather than the one the long-lived admin and producer clients are using, which
  sarama can rewrite in place.
- Cancelling the delete-topic dialog no longer reloads the topic list.

## [0.0.4] - 2026-04-19

### Added

- Session management: a startup session picker and an in-TUI session manager
  (`s`), with sessions persisted to disk and passwords held in memory only.
- Centralised theme and a consistent panel, tab bar, and status bar treatment
  across every view.

## [0.0.3] - 2025-09-04

### Added

- Consumer improvements: offset selection and message search.

[Unreleased]: https://github.com/digitalis-io/kconduit/compare/v0.0.4...HEAD
[0.0.4]: https://github.com/digitalis-io/kconduit/compare/v0.0.3...v0.0.4
[0.0.3]: https://github.com/digitalis-io/kconduit/compare/v0.0.2...v0.0.3

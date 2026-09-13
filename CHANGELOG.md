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

### Fixed

- `filterRows` returned the caller's own slice when no filter was applied, which
  `applyView` then sorted in place — quietly reordering the master row list each
  tab documents as unfiltered and unsorted. It now always returns a copy.
  (#9)

### Security

- `github.com/go-viper/mapstructure/v2` raised to v2.3.0, clearing GO-2025-3787
  (may leak sensitive information in logs when processing malformed data). It
  arrives transitively through viper and is reached from `viper.init`.

- Control characters are stripped from every value the cluster supplies —
  topic names, consumer group ids, ACL principals, and a consumer's `client.id`
  — before it reaches a table cell or the clipboard. A consumer can set its
  `client.id` to anything, including terminal escape sequences; unfiltered,
  those could rewrite the screen, and an OSC 52 sequence copied with `y` could
  overwrite the operator's clipboard.

### Changed

- `make` targets for the two test clusters: `kafka-up` / `run-plain` for the
  plaintext cluster and `kafka-acls-up` / `run-acls` for the SASL one, plus the
  matching `down`, `logs`, and `clean` targets for both. The two clusters
  publish overlapping ports, so each `up` target refuses to start while the
  other cluster is running rather than failing on a port bind.
- `make help` lists the targets again. The awk that generates it expected
  `target: ## description` while the file documents targets as
  `## target: description` on the preceding line, so it printed only the
  section headings.
- `run-local` and the three `test-ai-*` targets pointed at `localhost:19092`,
  where nothing listens: the plaintext cluster advertises 19094, 29094 and
  39094. `run-local` is now an alias for `run-plain`.
- `make` uses `docker compose` rather than the end-of-life `docker-compose` v1
  binary, and `run-dev` passes the SASL password through
  `KCONDUIT_SASL_PASSWORD` instead of the deprecated `--sasl-password` flag.
- The create-topic form is now a framed, centred dialog matching the rest of the
  app: labels in their own column, one line of guidance under the field being
  filled in, and a summary spelling out what Create will do with the defaults
  applied. Fields are validated as they are typed rather than one error at a
  time on submit, and Create stays dimmed until the form is complete. The
  replication factor is checked against the cluster's broker count, which is
  fetched as the form opens.
- The three ACL dialogs share that framing. They keep their huh forms, which do
  the operations multi-select well, but are now dressed in the application
  palette instead of hardcoded colours and carry a plain-English summary of the
  rule — "User:alice may read on topic \"orders\"" — rather than leaving the
  reader to assemble it from seven field names. The edit dialog spells out that
  it replaces the rule, because Kafka has no in-place update, and the delete
  dialog shows both the sentence and the exact field values it is about to
  remove.
- Gemini credentials are read from `GOOGLE_API_KEY` as well as
  `GEMINI_API_KEY`, so a shell already set up for the Google Cloud SDKs works
  without re-exporting. `GEMINI_API_KEY` wins when both are set.
- Sub-views are given the terminal size when they are opened. A dialog is
  created between resizes, so it never saw a window-size message of its own
  until the terminal happened to change, and was laid out for a zero-width
  screen until then.
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
- Cancelling the delete-topic dialog no longer reloads the topic list, and
  cancelling any of the three ACL dialogs no longer refetches the ACL list.

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

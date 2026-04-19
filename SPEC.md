# SPEC: Session Management UI

## 1. Objective

Add session management to the kconduit TUI so users can save, load, edit, and delete named Kafka connection profiles without retyping flags. Sessions are persisted in `~/.config/kconduit/sessions.yaml` via the existing `pkg/config` package.

**Target users:** Developers and operators who connect to multiple Kafka clusters (local dev, staging, production) and want fast switching without managing shell aliases.

---

## 2. Features and Acceptance Criteria

### 2.1 Startup Session Picker

**Trigger:** `main.go` — when neither `--brokers` nor `--session` flag is provided.

**Behaviour:**
- Run a standalone Bubble Tea program (`SessionPickerModel`) *before* Kafka connects.
- Display a table of saved sessions (name, brokers, SASL enabled, TLS enabled).
- If no sessions exist, open the New Session form directly.
- User actions:
  - `↑/↓` — navigate session list
  - `enter` — connect to selected session
  - `n` — create new session
  - `q` / `ctrl+c` — quit
- If the selected session has SASL enabled, show a password prompt before connecting (password held in memory only, never written to disk).
- On "connect": the picker exits and returns a `SessionConnect` value to `main.go`, which then uses it to connect to Kafka and launch the main TUI.

**Acceptance criteria:**
- [ ] Picker is shown when app starts with no `--brokers` / `--session` flags and at least one session exists
- [ ] New Session form is shown when app starts with no flags and no sessions exist
- [ ] Password prompt appears for SASL sessions; entered password is used for current connection only
- [ ] Selecting a session and pressing enter launches the main TUI connected to that cluster
- [ ] `q` quits cleanly

### 2.2 In-TUI Session Manager

**Trigger:** Press `s` from the main list view (BrokersTab, TopicsTab, ConsumerGroupsTab, or ACLsTab). Opens `SessionManagerView`.

**Behaviour — Session List screen:**
- Table columns: Name | Brokers | SASL | TLS | Active (✓ if currently connected)
- Key bindings:
  - `↑/↓` — navigate
  - `n` — new session
  - `e` — edit selected session
  - `d` — delete selected session (confirmation dialog)
  - `c` — connect to selected session
  - `esc` — return to main TUI

**Behaviour — Create/Edit form:**
- Built with `huh` (same pattern as `CreateACLHuhModel`)
- Fields grouped into pages:
  - Page 1 — Connection: Name (text), Brokers (text)
  - Page 2 — Auth: SASL Enabled (bool), Mechanism (select: PLAIN/SCRAM-SHA-256/SCRAM-SHA-512), Username (text), Protocol (select: SASL_PLAINTEXT/SASL_SSL), Password (text, masked — in-memory only, not saved)
  - Page 3 — TLS: TLS Enabled (bool), CA Cert path (text), Client Cert path (text), Client Key path (text), Skip Verify (bool)
  - Page 4 — Advanced: Log Level (select), AI Engine (select), AI Model (text)
- Password field is present in the form but **never written** to the YAML file
- On save: `config.Save(name, session)` is called (without password)
- If editing the currently active session: automatically trigger reconnect after saving

**Behaviour — Delete confirmation:**
- Simple inline confirmation: "Delete session `<name>`? [y/N]"
- On confirm: `config.Delete(name)`, return to session list

**Behaviour — Connect:**
- If session has SASL: show a password prompt overlay (single `textinput`, masked)
- On confirm: restart the process via `syscall.Exec(os.Args[0], []string{os.Args[0], "--session", name}, env)` where `env` includes `KCONDUIT_SASL_PASSWORD=<entered password>`
- If no SASL: restart immediately with `--session <name>`

**Acceptance criteria:**
- [ ] `s` from main view opens session manager
- [ ] Session table shows all saved sessions; active session marked with ✓
- [ ] `n` opens create form with all four pages
- [ ] `e` on a session opens edit form pre-filled with existing values (password field empty)
- [ ] `d` shows inline confirmation; confirmed delete removes session from list and file
- [ ] `c` on a non-SASL session restarts the process with `--session <name>`
- [ ] `c` on a SASL session prompts for password, then restarts with session + password in env
- [ ] Editing the active session and saving triggers automatic reconnect
- [ ] `esc` from any sub-view returns to session list; `esc` from session list returns to main TUI
- [ ] Password is never written to the YAML file

---

## 3. Architecture

### 3.1 New `SessionConnect` type

```go
// pkg/ui/session_picker.go
type SessionConnect struct {
    Name     string
    Session  config.Session
    Password string // in-memory only
}
```

Returned by the startup picker to `main.go`. Used to build `kafka.SASLConfig`.

### 3.2 New files

| File | Purpose |
|------|---------|
| `pkg/ui/session_picker.go` | Standalone Bubble Tea program for startup session selection |
| `pkg/ui/session_manager.go` | `SessionManagerModel` — in-TUI session list, delete, connect |
| `pkg/ui/session_form.go` | `SessionFormModel` — huh-based create/edit form (shared) |

### 3.3 Changes to existing files

| File | Change |
|------|--------|
| `pkg/ui/model.go` | Add `SessionManagerView` to `ViewMode`; add `sessionManagerModel SessionManagerModel` field; handle `s` key in `updateListView`; add `updateSessionManagerView` dispatch |
| `cmd/kconduit/main.go` | Before connecting: if no `--brokers`/`--session`, run `RunSessionPicker()` to get `SessionConnect`; use `SessionConnect.Password` to build `SASLConfig` |

### 3.4 Reconnect mechanism

Reconnect (connect from in-TUI manager, or save edit of active session) uses `syscall.Exec`:

```go
func reconnectWithSession(name, password string) error {
    env := os.Environ()
    if password != "" {
        env = append(env, "KCONDUIT_SASL_PASSWORD="+password)
    }
    return syscall.Exec(os.Args[0], []string{os.Args[0], "--session", name}, env)
}
```

`syscall.Exec` replaces the current process in-place — the terminal sees no restart, the TUI closes and re-opens seamlessly.

### 3.5 `main.go` startup flow

```
main()
  ├─ parse flags
  ├─ if --brokers or --session provided:
  │    └─ proceed as today (connect, launch main TUI)
  └─ else:
       ├─ run RunSessionPicker() → SessionConnect (or quit)
       └─ use SessionConnect to connect + launch main TUI
```

`RunSessionPicker()` is a blocking call that runs a separate `tea.Program` and returns the selected `SessionConnect`.

---

## 4. Code Style

- Follow existing patterns:
  - Sub-models use `(m Model) Init() tea.Cmd`, `Update`, `View` — same as `DeleteTopicModel`, `CreateACLHuhModel`
  - Forms use `github.com/charmbracelet/huh` with a `*huh.Form` field — same as `CreateACLHuhModel`
  - Styling via `lipgloss` — reuse existing style variables in `model.go` where possible
  - All Kafka/config operations run as `tea.Cmd` (never blocking in `Update`)
- `SessionManagerModel` is embedded in the main `Model` struct (not a pointer, consistent with `DeleteTopicModel`)
- `SessionFormModel` is a pointer field (consistent with `CreateACLHuhModel`) since the huh form carries mutable state
- Error messages follow existing pattern: set `m.err`, render inline in `View`

---

## 5. Testing Strategy

- `pkg/ui/session_picker.go` and `pkg/ui/session_manager.go`: unit tests for `Update` message handling using `bubbletea/teatest` or direct message injection
- `pkg/ui/session_form.go`: unit test that form produces a correct `SessionConnect` on completion
- `pkg/config` package already has full unit test coverage — no changes needed there
- No integration tests (Kafka connection not required for UI tests)

---

## 6. Boundaries

### Always do
- Password held in `SessionConnect.Password` in memory; never passed to `config.Save`
- `config.Save` called with a `Session` where `SASL.PasswordFile` is empty when password was entered inline (the two mechanisms are mutually exclusive per session)
- `syscall.Exec` is the sole reconnect mechanism — no goroutine-based reconnect, no manual client swap

### Ask first about
- Any change to the `config.Session` struct that adds new fields
- Changes to `pkg/config` package behaviour
- Changing the `s` keybinding (could conflict with future bindings)

### Never do
- Write a plaintext password to any file
- Block the Bubble Tea update loop with synchronous I/O (file reads, Kafka calls must be `tea.Cmd`)
- Use `os.Exit` anywhere in the UI packages (use `tea.Quit` instead)
- Modify the existing `--session` / `--save-session` / `--list-sessions` CLI flag behaviour

---

## 7. Open Questions (resolved)

| Question | Answer |
|----------|--------|
| Startup when no sessions exist | Show New Session form directly |
| Password storage | In-memory only; form field present but value not persisted |
| Connect mechanism | `syscall.Exec` — process restart in-place |
| Reconnect on editing active session | Automatic after save |
| Session list style | Table (consistent with existing UI) |
| In-TUI key binding | `s` from main list view |

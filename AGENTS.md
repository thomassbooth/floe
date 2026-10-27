# AGENTS.md

Notes for coding agents working on floe, a terminal multiplexer with
free-floating windows. Read [README.md](README.md) for what it does and
[ARCHITECTURE.md](ARCHITECTURE.md) for how the code is laid out.

## Commands

```sh
make build        # build ./floe (gitignored)
make test         # unit, architecture and end-to-end tests
make test-short   # skip the slower end-to-end tests
make check        # everything CI runs: gofmt, vet, unit tests with -race, e2e
make help         # every target
```

CI runs `make check`'s steps on both macOS and Linux, so run it before
calling a change done. `install.sh` is the one-line installer the README
points to; run `shellcheck install.sh` after changing it.

## Layout

| Path | What |
|---|---|
| `cmd/floe` | Composition root: `floe` runs the client, `floe server` the server |
| `internal/domain` | Sessions, windows, layout rules. Imports nothing |
| `internal/service` | One actor goroutine that owns all state, plus the ports it needs |
| `internal/ui` | Bubble Tea client: windows, status bar, overlays |
| `internal/remote` | Bridges the `ui.Backend` port and the service over a Unix socket |
| `internal/protocol` | Messages between client and server, gob encoded |
| `internal/infra/*` | Adapters to the OS: PTYs, files, settings, the socket |
| `internal/input`, `theme`, `queue` | Pure helpers |
| `internal/archtest` | Fails the tests if an import breaks the layering |
| `e2e` | Runs the real binary in a pseudo-terminal and checks the screen |

## Rules that the code relies on

- **Layering.** Imports point inward, towards the domain. Every package needs
  an entry in `rules` in `internal/archtest/arch_test.go`, listing the
  project packages it may import and whether it may use third-party code.
  A new package without one fails the tests. Only `ui`, `infra/terminal` and
  `cmd/floe` may import third-party libraries.
- **The actor owns the state.** `Service.Run` is the only goroutine that
  touches service state. Other goroutines send it requests or signals; they
  never reach into it.
- **Ports, not implementations.** The service talks to the outside through
  interfaces in `internal/service/ports.go`, and the UI through
  `ui.Backend`. Keep new OS or process concerns behind a port.
- **New commands and events cross the wire.** Anything that goes between
  client and server must be registered in `internal/protocol/register.go`.
- **Client and server must match.** A server refuses a client from a
  different build. After rebuilding, run `floe kill-server` before trying
  the new binary by hand.

## Testing

- Bug fixes start with a test that reproduces the bug the way a user would
  see it, usually an end-to-end test in `e2e/`. Then fix it.
- End-to-end tests build the binary once in `TestMain` and give each test
  its own `XDG_CONFIG_HOME`, `XDG_STATE_HOME` and `XDG_RUNTIME_DIR`, so they
  never touch your real settings, sessions or server. Use `launch` or
  `launchWith` in `e2e/e2e_test.go` rather than starting floe another way.
- Service tests use the fake host in `internal/service/harness_test.go`.
- Wait for what should appear on screen (`waitFor` and friends); do not
  sleep for a fixed time. A flaky test is a bug, so fix it rather than
  retrying.
- Keep both platforms working: PTY, process and socket code has
  `_darwin.go` and `_linux.go` variants.

## Style

- Match the surrounding code. Comments are full sentences in plain English
  and explain why or what something is for, not how.
- Doc comments on every exported identifier, and on unexported ones whose
  purpose is not obvious.
- In prose, docs and commit messages, use a plain dash `-`, never an em dash.
- Keep `README.md` and `ARCHITECTURE.md` in step with behavior: new keys go
  in the README tables, new packages in both the layout table above and the
  architecture diagrams.

## Commits and releases

- Subject: one imperative sentence about what changes for the user, such as
  "Detach with ctrl+b d" or "Let a failed release be re-run". No prefixes
  like `feat:`, no trailing period.
- Body, when needed: plain prose on what the user sees now and why.
- Do not add an agent as co-author.
- Releases are made by pushing a `v*` tag. GoReleaser builds the archives,
  publishes the GitHub release and updates the Homebrew tap. Try it locally
  with `make snapshot`.

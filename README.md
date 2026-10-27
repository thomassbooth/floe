# floe

A terminal multiplexer with free-floating windows. Drag them, resize them,
snap them to the edges of the screen, or tidy them into a grid with one key.
Sessions run in a background server, so closing your terminal never stops
your programs: run `floe` again and everything is where you left it.

```
╭─ 1 sh ──────────────────────── □ × ─╮╭─ 2 sh ──────────────────────── □ × ─╮
│-sh-3.2$ echo hello from floe        ││-sh-3.2$ ls /                        │
│hello from floe                      ││Applications    private              │
│-sh-3.2$                             ││bin             run                  │
│                                     ││cores           sbin                 │
│                                     ││dev             System               │
│                                     ││etc             tmp                  │
│                                     ││home            Users                │
│                                     ││Library         usr                  │
│                                     ││nix             var                  │
│                                     ││opt             Volumes              │
│                                     ││-sh-3.2$                             │
╰─────────────────────────────────────╯╰─────────────────────────────────────╯
 work   1 sh  2 sh                                    arrange   ctrl+b prefix
```

## Features

- **Free windows.** Drag a title bar to move a window, drag an edge or corner
  to resize it. Each title bar has maximize (□) and close (×) buttons.
- **Snapping.** Drop a window on the left or right edge to fill that half of
  the screen, or near the top or bottom of an edge for a quarter. `ctrl+b =` tidies every window
  into a layout that fills the screen.
- **Arrange mode.** Move and resize windows from the keyboard, or by clicking
  `arrange` in the status bar.
- **Sessions that survive.** Detach with `ctrl+b d` or just close the
  terminal; programs keep running in the background server. Layouts and
  working directories are also saved to disk, so they come back after a
  reboot (with fresh shells).
- **Several sessions.** Name them, switch between them from a list, and
  attach to the same session from more than one terminal.
- **Mouse everywhere.** Click a tab to focus its window, middle-click a tab to
  close it, scroll back through history with the wheel, and select text to
  copy it. Programs that use the mouse, such as editors, get it too.
- **Clipboard.** Programs can copy to your system clipboard (OSC 52), even
  over SSH.
- **Live titles.** Windows are named after the program running in them, and
  tabs are marked when a window you are not looking at has new output (•) or
  rings the bell (!).
- **Themes.** Eight built in, with a live preview picker: Violet, Catppuccin
  Mocha and Latte, Tokyo Night, Gruvbox, Nord, Rosé Pine and Dracula. Until
  you pick one, floe matches your terminal's light or dark background.
- **Help when you need it.** Press the prefix and a popup lists every key.

## Install

floe runs on macOS and Linux. Building it needs Go 1.26 or newer.

```sh
git clone https://github.com/thomassbooth/floe.git
cd floe
go build -o floe ./cmd/floe
```

Then put `floe` somewhere on your `PATH`.

For the best results use a terminal with true color and mouse support, such
as WezTerm, Ghostty, kitty, iTerm2 or Alacritty.

## Usage

```sh
floe               # attach to the server, starting it if needed
floe kill-server   # stop the server and every program in it; layouts are kept
floe help
```

Everything else happens inside floe. The prefix key is `ctrl+b`, as in tmux:
press it, let go, then press the key for what you want. Press `ctrl+b`
twice to send `ctrl+b` itself to the program in the window.

### Windows

| Keys | Action |
|---|---|
| `c` | New window, centered, half the size of the terminal |
| `\|` or `%` | Split the focused window, new window on the right |
| `-` or `"` | Split the focused window, new window below |
| `h` `j` `k` `l` or arrows | Move focus to the window in that direction |
| `H` `J` `K` `L` or shift+arrows | Resize the focused window (repeatable) |
| `=` | Tidy all windows into a layout that fills the screen |
| `z` | Maximize the focused window, or put it back |
| `x` | Close the focused window |
| `a` | Enter arrange mode |

### Sessions

| Keys | Action |
|---|---|
| `s` | List sessions and switch between them |
| `C` | New session |
| `$` | Rename the current session |
| `(` `)` | Previous or next session |
| `d` | Detach: leave everything running and return to your shell |
| `q` | Kill the current session and every program in it |

### General

| Keys | Action |
|---|---|
| `t` | Pick a theme, with a live preview |

### Arrange mode

Arrange mode takes over the keyboard until you leave it, so you can lay out
windows without pressing the prefix each time.

| Keys | Action |
|---|---|
| `h` `j` `k` `l` or arrows | Move the focused window |
| `H` `J` `K` `L` or shift+arrows | Resize the focused window |
| `tab` / `shift+tab` | Focus the next or previous window |
| `=` `z` `c` `\|` `-` `x` | Tidy, maximize, new window, split, close |
| `esc`, `enter`, `a` or `q` | Done |

### Mouse

| Do this | To |
|---|---|
| Drag a title bar | Move the window |
| Drag an edge or corner | Resize the window |
| Drop a window on the left or right edge | Snap it to that half, or a quarter near the top or bottom |
| Click □ / × in a title bar | Maximize / close the window |
| Click a tab in the status bar | Focus that window |
| Middle-click a tab | Close that window |
| Click the session name | Open the session list |
| Scroll the wheel | Scroll back through the window's history |
| Drag over text | Select it and copy it to the clipboard |
| Double- / triple-click | Select a word / a line |

## Files

| Path | What |
|---|---|
| `~/.config/floe/settings.json` | Your chosen theme |
| `~/.local/state/floe/sessions.json` | Saved sessions, for after a restart |
| `~/.local/state/floe/server.log` | What the server has to say |
| `$TMPDIR/floe-<uid>/` | The server's socket, private to you |

`XDG_CONFIG_HOME`, `XDG_STATE_HOME` and `XDG_RUNTIME_DIR` are honored.

## After upgrading

A server only talks to clients built from the same binary. If you rebuild
floe while a server is running, `floe` will tell you so; run
`floe kill-server` and then `floe` again. Your windows and layouts come back,
but programs that were running in them do not.

## Development

```sh
go test ./...          # unit, architecture and end-to-end tests
go test -short ./...   # skip the slower end-to-end tests
```

The end-to-end tests run the real binary in a pseudo-terminal, each with its
own server, settings and state, so they never touch yours. See
[ARCHITECTURE.md](ARCHITECTURE.md) for how the code is laid out.

## License

MIT, see [LICENSE](LICENSE).

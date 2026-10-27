# floe

A terminal multiplexer with free-floating windows. Drag them, resize them,
snap them to the edges of the screen, or tidy them into a grid with one key.
Sessions run in a background server, so closing your terminal never stops
your programs: run `floe` again and everything is where you left it.

<img width="1189" height="695" alt="image" src="https://github.com/user-attachments/assets/4be1d43d-c097-401f-b738-1fb77e90bee3" />


## Features

- **Free windows.** In arrange mode, drag a title bar to move a window and
  drag an edge or corner to resize it. Each title bar has minimize (_),
  maximize (□) and close (×) buttons.
- **Minimize.** Tuck a window away with `ctrl+b m` while its program keeps
  running. Its tab stays in the status bar, in italics, still showing when
  it is busy or rings the bell; click it or press `ctrl+b M` to bring the
  window back.
- **Snapping.** Drop a window on the left or right edge to fill that half of
  the screen, or near the top or bottom of an edge for a quarter. `ctrl+b =` tidies every window
  into a layout that fills the screen.
- **Arrange mode.** Move and resize windows from the keyboard or the mouse.
  Turn it on with `ctrl+b a` or by clicking `arrange` in the status bar.
- **Smooth.** Windows glide into place when you split, tidy, snap, maximize
  or open them. Turn it off in `floe.conf`.
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
- **Live titles.** Windows are named after the program running in them.
  A tab shows a dot (•) while its program is busy printing, such as a build
  or an agent at work, and a mark (!) when a window you are not looking at
  rings the bell.
- **Themes.** Eight built in, with a live preview picker: Violet, Catppuccin
  Mocha and Latte, Tokyo Night, Gruvbox, Nord, Rosé Pine and Dracula. Until
  you pick one, floe matches your terminal's light or dark background.
- **Help when you need it.** Press the prefix and a popup lists every key.
- **Your own keys.** Change the prefix and any binding in a small config
  file, in the style of tmux.

## Install

floe runs on macOS and Linux.

**macOS, with Homebrew:**

```sh
brew install thomassbooth/tap/floe
```

**Linux or macOS, with one command:**

```sh
curl -fsSL https://raw.githubusercontent.com/thomassbooth/floe/main/install.sh | sh
```

It downloads the latest release for your system, checks it against the
release's checksums and installs it to `/usr/local/bin`, or `~/.local/bin`
without sudo. Set `FLOE_VERSION=v0.2.0` for a particular release or
`FLOE_INSTALL_DIR` for somewhere else. Run it again to upgrade.

**Linux, with your package manager:** download the package for your system
from the [latest release](https://github.com/thomassbooth/floe/releases/latest)
and install it. Use `amd64` for most PCs and `arm64` for ARM machines such as a
Raspberry Pi.

```sh
sudo apt install ./floe_<version>_amd64.deb              # Debian, Ubuntu
sudo dnf install ./floe_<version>_amd64.rpm              # Fedora, RHEL
sudo apk add --allow-untrusted floe_<version>_amd64.apk  # Alpine
```

**By hand:** download the archive for your system from the latest release,
unpack it and put `floe` somewhere on your `PATH`.

**With Go** 1.26 or newer:

```sh
go install github.com/thomassbooth/floe/cmd/floe@latest
```

**From source:**

```sh
git clone https://github.com/thomassbooth/floe.git
cd floe
make && sudo make install   # or: make install PREFIX=~/.local
```

For the best results use a terminal with true color and mouse support, such
as WezTerm, Ghostty, kitty, iTerm2 or Alacritty.

## Usage

```sh
floe               # attach to the server, starting it if needed
floe kill-server   # stop the server and every program in it; layouts are kept
floe keys          # list every key binding and the name to change it by
floe help
```

Everything else happens inside floe. The prefix key is `ctrl+b`, as in tmux
(see [Custom keys](#custom-keys) to change it):
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
| `m` | Minimize the focused window; its program keeps running |
| `M` | Bring back the most recently minimized window |
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
| `=` `z` `m` `M` | Tidy, maximize, minimize, restore |
| `c` `\|` `-` `x` | New window, split, close |
| `esc`, `enter`, `a` or `q` | Done |

### Custom keys

Put your own keys in `~/.config/floe/keybinds.conf`. Each line is one
command, and lines starting with `#` are comments:

```sh
# Use ctrl+a, like screen.
prefix ctrl+a

# Split with v and s instead.
bind v split-right
bind s split-down
unbind |
unbind %

# Keys in arrange mode take --arrange.
bind --arrange w move-up
```

| Command | Does |
|---|---|
| `prefix <key>` | Use another prefix key |
| `bind <key> <action>` | Run an action with a key after the prefix. The key leaves whatever it ran before, and the action keeps its other keys |
| `unbind <key>` | Make a key do nothing after the prefix |
| `unbind all` | Remove every binding, to start from nothing |
| `bind --arrange ...`, `unbind --arrange ...` | The same, for arrange mode |

Keys are written as `ctrl+a`, `alt+x`, `shift+left`, `H`, `|`, `space`, `tab`,
`enter`, `esc` or `f1` to `f12`. Run `floe keys` to see every action's
name and current keys; some, such as `focus-next`, have no key until you give
them one. Later lines win.

floe reads the file each time you run `floe`, so detach and run it again
after a change. If a line is wrong, floe says which in the status bar and
uses the default keys until it is fixed.

### Settings

Other settings go in `~/.config/floe/floe.conf`, one per line, in the same
style as `keybinds.conf`:

```sh
# Windows jump into place instead of gliding.
animations off
```

| Setting | Values | Default |
|---|---|---|
| `animations` | `on` or `off` | `on`, or `off` over SSH where frames can arrive late |

floe reads it each time you run `floe`. If a line is wrong, floe says which
in the status bar and uses the defaults until it is fixed.

### Mouse

| Do this | To |
|---|---|
| Drag a title bar, in arrange mode | Move the window |
| Drag an edge or corner, in arrange mode | Resize the window |
| Drop a window on the left or right edge | Snap it to that half, or a quarter near the top or bottom |
| Click _ / □ / × in a title bar | Minimize / maximize / close the window |
| Click a tab in the status bar | Focus that window, bringing it back if minimized |
| Middle-click a tab | Close that window |
| Click the session name | Open the session list |
| Scroll the wheel | Scroll back through the window's history |
| Drag over text | Select it and copy it to the clipboard |
| Double- / triple-click | Select a word / a line |

## Files

| Path | What |
|---|---|
| `~/.config/floe/settings.json` | Your chosen theme |
| `~/.config/floe/keybinds.conf` | Your own keys, if you write one |
| `~/.config/floe/floe.conf` | Other settings, such as animations |
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
make test         # unit, architecture and end-to-end tests
make test-short   # skip the slower end-to-end tests
make check        # everything CI runs
make help         # every target
```

CI runs every test on macOS and Linux. To release, push a tag such as
`v0.1.0`: GoReleaser builds the archives, publishes a GitHub release and
updates the Homebrew tap.

The end-to-end tests run the real binary in a pseudo-terminal, each with its
own server, settings and state, so they never touch yours. See
[ARCHITECTURE.md](ARCHITECTURE.md) for how the code is laid out.

## License

MIT, see [LICENSE](LICENSE).

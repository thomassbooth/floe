# Architecture

floe is two processes. The **server** runs in the background and owns every
session, window and program. The **client** is what you run in a terminal: it
draws what the server sends and passes on your keys and mouse. Closing the
terminal only ends the client, so programs keep running.

## The two processes

```mermaid
flowchart LR
    you(["You: keys and mouse"])

    subgraph client["floe - client process, one per terminal"]
        direction TB
        ui["<b>ui</b><br/>Bubble Tea: draws windows,<br/>status bar, overlays"]
        input["<b>input</b><br/>prefix key, arrange mode,<br/>your own keys"]
        theme["<b>theme</b><br/>colors"]
        settings[("<b>settings</b><br/>settings.json,<br/>floe.conf, keybinds.conf")]
        rclient["<b>remote.Client</b><br/>stands in for the service"]
        ui --> input
        ui --> theme
        ui --> settings
        ui -- "ui.Backend port" --> rclient
    end

    subgraph server["floe server - background process, outlives the terminal"]
        direction TB
        rserver["<b>remote.Server</b><br/>one service client per connection"]
        service["<b>service</b><br/>actor: applies commands,<br/>publishes events"]
        domain["<b>domain</b><br/>sessions, windows, layout rules"]
        terminal["<b>terminal</b><br/>shells, PTYs, screens"]
        store[("<b>store</b><br/>sessions.json")]
        rserver --> service
        service --> domain
        service -- "PaneHost port" --> terminal
        service -- "SessionStore port" --> store
    end

    you --> ui
    rclient <==>|"Unix socket<br/>messages encoded by <b>protocol</b>"| rserver
    socket["<b>socket</b><br/>where it lives,<br/>lock file"] -.- rserver
```

## The layers

Arrows point the way imports go: inward, towards the domain. Nothing inside
knows about what is outside it. `internal/archtest` fails the tests if an
import breaks this.

```mermaid
flowchart TB
    subgraph root["Composition root"]
        cmd["<b>cmd/floe</b><br/>builds and connects everything"]
    end

    subgraph adaptersIn["Adapters in: drive the service"]
        ui["<b>ui</b>"]
        remote["<b>remote</b>"]
    end

    subgraph adaptersOut["Adapters out: talk to the OS"]
        terminal["<b>terminal</b>"]
        store["<b>store</b>"]
        settings["<b>settings</b>"]
        socket["<b>socket</b>"]
    end

    subgraph core["Core"]
        service["<b>service</b>"]
        domain["<b>domain</b><br/>imports nothing"]
    end

    subgraph helpers["Pure helpers"]
        protocol["<b>protocol</b>"]
        input["<b>input</b>"]
        theme["<b>theme</b>"]
        queue["<b>queue</b>"]
    end

    cmd --> ui & remote & service & terminal & store & settings & socket
    ui --> input & theme & domain
    remote --> protocol & queue & domain
    terminal --> domain & queue
    store --> domain
    service --> domain & queue
    protocol --> domain
    input --> domain

    classDef coreStyle fill:#ede9fe,stroke:#7c3aed,color:#1e1b4b
    classDef inStyle fill:#e0f2fe,stroke:#0284c7,color:#0c4a6e
    classDef outStyle fill:#dcfce7,stroke:#16a34a,color:#14532d
    classDef helperStyle fill:#fef3c7,stroke:#d97706,color:#451a03
    class service,domain coreStyle
    class ui,remote inStyle
    class terminal,store,settings,socket outStyle
    class protocol,input,theme,queue helperStyle
```

| Layer | Packages | Job |
|---|---|---|
| Domain | `domain` | What sessions and windows are, and the rules for them |
| Service | `service` | Runs the domain: one actor goroutine, per-responsibility services, ports |
| Adapters in | `ui`, `remote` | Drive the service: the terminal UI, and the socket bridge |
| Adapters out | `terminal`, `store`, `settings`, `socket` | Implement ports with the OS: PTYs, files, sockets |
| Helpers | `protocol`, `input`, `theme`, `queue` | Pure building blocks |
| Root | `cmd/floe` | `floe` runs the client, `floe server` the server |

## A key press, end to end

```mermaid
sequenceDiagram
    autonumber
    actor You
    participant UI as ui
    participant RC as remote.Client
    participant RS as remote.Server
    participant S as service
    participant T as terminal (shell)

    You->>UI: press "ls" + Enter
    UI->>RC: Apply(SendKey)
    RC->>RS: protocol.Apply, as bytes
    RS->>S: Apply(SendKey)
    S->>T: SendKey
    T-->>S: screen changed
    S-->>RS: Changed signal
    RS-->>RC: protocol.Changed
    RC-->>UI: Changed()
    UI->>RC: Grid(pane)
    RC->>RS: protocol.GridRequest
    RS->>S: Grid(pane)
    RS-->>RC: protocol.GridReply, compact screen bytes
    RC-->>UI: grid
    UI-->>You: redraw
```

## Why the split was cheap

The UI only knows the `ui.Backend` port, and the service only knows its own
clients. Putting `remote.Client` behind the port and `remote.Server` in front
of the service moved the service into another process without changing
either of them.

## Where new things go

| To add | Start in |
|---|---|
| A panel that takes the keyboard, such as a picker, a prompt or copy mode | `ui/overlay.go`: implement `overlay` and open it from `runAction` |
| Something the mouse does | `ui/mouse.go`, which routes presses to `hittest.go`, `drag.go`, `selection.go` and `scroll.go` |
| A key binding | `input/keymap.go`; its name then works in `keybinds.conf` |
| A command or event between client and server | `domain/command.go` or `event.go`, then `protocol/register.go` |
| A window rule, such as how windows snap | `domain/session_arrange.go`, or `session_focus.go` for focus and marks |
| A field saved with a window | `domain.SavedWindow` and the file format in `infra/store`. The round-trip tests in both fail until they give the new field a value |

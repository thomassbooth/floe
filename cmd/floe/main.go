// Command floe is a terminal multiplexer.
//
// Running floe attaches to the floe server, starting it first if needed. The
// server runs in the background and owns the sessions and the programs in
// them, so they keep running when the terminal closes.
package main

import (
	"fmt"
	"os"
)

const usage = `usage: floe [command]

With no command, attach to the floe server, starting it if needed.

Commands:
  kill-server   stop the server and every program in it; layouts are kept
  help          show this help
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "floe:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return runClient()
	}
	switch args[0] {
	case "server":
		// Started by the client; not for people to run.
		return runServer(args[1:])
	case "kill-server":
		return killServer()
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	}
	fmt.Fprint(os.Stderr, usage)
	return fmt.Errorf("unknown command %q", args[0])
}

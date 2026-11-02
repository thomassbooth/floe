// Package opener hands a link to the program the desktop opens links with.
package opener

import (
	"fmt"
	"os/exec"
)

// Open opens a URL with the desktop's own program, and waits for that program
// to finish. The URL is passed as one argument, so no shell ever sees it.
func Open(url string) error {
	if err := exec.Command(program, url).Run(); err != nil {
		return fmt.Errorf("open link: %w", err)
	}
	return nil
}

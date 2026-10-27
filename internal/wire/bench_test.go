package wire

import (
	"testing"

	"github.com/thomasbooth/multiplexer/internal/domain"
)

// BenchmarkGridRoundTrip measures a request and reply for a large,
// colorful screen, the most common traffic while drawing.
func BenchmarkGridRoundTrip(b *testing.B) {
	size := domain.Size{W: 240, H: 60}
	grid := domain.Grid{Size: size, Cells: make([]domain.Cell, size.W*size.H)}
	for i := range grid.Cells {
		grid.Cells[i] = domain.Cell{Content: string(rune('a' + i%26)), Width: 1, Style: domain.Style{Fg: domain.IndexedColor(uint8(i / 7))}}
	}
	client, server := pipe(b)
	go func() {
		for {
			m, err := server.Receive()
			if err != nil {
				return
			}
			_ = server.Send(GridReply{ID: m.(GridRequest).ID, Grid: grid, OK: true})
		}
	}()
	b.ResetTimer()
	for i := range b.N {
		if err := client.Send(GridRequest{ID: uint64(i)}); err != nil {
			b.Fatal(err)
		}
		if _, err := client.Receive(); err != nil {
			b.Fatal(err)
		}
	}
}

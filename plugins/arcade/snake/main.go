// Snake for a terminal pane: arrows, WASD or hjkl steer, a mouse click turns toward it, p pauses,
// r restarts, q quits. Standard library only, so `go run ./snake` is all it needs.
package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type pt struct{ x, y int }

type game struct {
	w, h   int
	snake  []pt
	dir    pt
	next   pt
	food   pt
	score  int
	over   bool
	paused bool
}

func newGame(w, h int) *game {
	g := &game{w: w, h: h, dir: pt{1, 0}, next: pt{1, 0}}
	for i := 3; i >= 0; i-- {
		g.snake = append(g.snake, pt{w/2 - i, h / 2})
	}
	g.place()
	return g
}

func (g *game) head() pt { return g.snake[len(g.snake)-1] }

func (g *game) on(p pt) bool {
	for _, s := range g.snake {
		if s == p {
			return true
		}
	}
	return false
}

func (g *game) place() {
	for {
		g.food = pt{rand.IntN(g.w), rand.IntN(g.h)}
		if !g.on(g.food) {
			return
		}
	}
}

// turn ignores a reversal, which would run the snake into itself.
func (g *game) turn(d pt) {
	if d.x != -g.dir.x || d.y != -g.dir.y {
		g.next = d
	}
}

func (g *game) step() {
	if g.over || g.paused {
		return
	}
	g.dir = g.next
	h := g.head()
	n := pt{h.x + g.dir.x, h.y + g.dir.y}
	if n.x < 0 || n.y < 0 || n.x >= g.w || n.y >= g.h || g.on(n) {
		g.over = true
		return
	}
	g.snake = append(g.snake, n)
	if n == g.food {
		g.score++
		g.place()
		return
	}
	g.snake = g.snake[1:]
}

func (g *game) draw() string {
	var b strings.Builder
	b.WriteString("\x1b[H")
	status := fmt.Sprintf(" snake  score %d   arrows/wasd/click steer · p pause · r restart · q quit", g.score)
	if g.over {
		status = fmt.Sprintf(" game over, score %d   r restart · q quit", g.score)
	}
	b.WriteString("\x1b[7m" + pad(status, g.w*2+2) + "\x1b[0m\r\n")
	b.WriteString("┌" + strings.Repeat("──", g.w) + "┐\r\n")
	cells := map[pt]string{g.food: "\x1b[31m●\x1b[0m "}
	for i, s := range g.snake {
		cells[s] = "\x1b[32m█\x1b[0m█"
		if i == len(g.snake)-1 {
			cells[s] = "\x1b[92m██\x1b[0m"
		}
	}
	for y := 0; y < g.h; y++ {
		b.WriteString("│")
		for x := 0; x < g.w; x++ {
			if c, ok := cells[pt{x, y}]; ok {
				b.WriteString(c)
			} else {
				b.WriteString("  ")
			}
		}
		b.WriteString("│\r\n")
	}
	b.WriteString("└" + strings.Repeat("──", g.w) + "┘")
	return b.String()
}

func pad(s string, n int) string {
	if r := []rune(s); len(r) < n {
		return s + strings.Repeat(" ", n-len(r))
	}
	return s
}

var mouseRE = regexp.MustCompile(`\x1b\[<(\d+);(\d+);(\d+)M`)

// input turns raw bytes into turns, commands and SGR mouse clicks (column, row from 1).
func (g *game) input(b []byte) (quit bool) {
	s := string(b)
	for _, m := range mouseRE.FindAllStringSubmatch(s, -1) {
		if m[1] != "0" {
			continue
		}
		col, _ := strconv.Atoi(m[2])
		row, _ := strconv.Atoi(m[3])
		// the board starts at row 3, column 2, two columns per cell
		c := pt{(col - 2) / 2, row - 3}
		h := g.head()
		dx, dy := c.x-h.x, c.y-h.y
		if abs(dx) > abs(dy) {
			g.turn(pt{sign(dx), 0})
		} else if dy != 0 {
			g.turn(pt{0, sign(dy)})
		}
	}
	s = mouseRE.ReplaceAllString(s, "")
	for _, k := range []struct {
		keys []string
		d    pt
	}{
		{[]string{"\x1b[A", "w", "k"}, pt{0, -1}},
		{[]string{"\x1b[B", "s", "j"}, pt{0, 1}},
		{[]string{"\x1b[C", "d", "l"}, pt{1, 0}},
		{[]string{"\x1b[D", "a", "h"}, pt{-1, 0}},
	} {
		for _, key := range k.keys {
			if strings.Contains(s, key) {
				g.turn(k.d)
			}
		}
	}
	switch {
	case strings.ContainsAny(s, "q\x03"):
		return true
	case strings.Contains(s, "p"):
		g.paused = !g.paused
	case strings.Contains(s, "r"):
		*g = *newGame(g.w, g.h)
	}
	return false
}

func abs(n int) int { return max(n, -n) }

func sign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}

func stty(args ...string) string {
	c := exec.Command("stty", args...)
	c.Stdin = os.Stdin
	out, _ := c.Output()
	return strings.TrimSpace(string(out))
}

func main() {
	rows, cols := 24, 80
	if f := strings.Fields(stty("size")); len(f) == 2 {
		rows, _ = strconv.Atoi(f[0])
		cols, _ = strconv.Atoi(f[1])
	}
	g := newGame(max(10, (cols-2)/2), max(5, rows-3))
	stty("raw", "-echo")
	fmt.Print("\x1b[?25l\x1b[?1000h\x1b[?1006h\x1b[2J")
	defer func() {
		fmt.Print("\x1b[?1000l\x1b[?1006l\x1b[?25h\x1b[2J\x1b[H")
		stty("sane")
	}()
	keys := make(chan []byte)
	go func() {
		for {
			buf := make([]byte, 64)
			n, err := os.Stdin.Read(buf)
			if err != nil {
				close(keys)
				return
			}
			keys <- buf[:n]
		}
	}()
	tick := time.NewTicker(110 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case b, ok := <-keys:
			if !ok || g.input(b) {
				return
			}
		case <-tick.C:
			g.step()
		}
		fmt.Print(g.draw())
	}
}

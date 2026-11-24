// barkeep draws a strip of the machine's readings down the side of a
// terminal, over a slow loop of colour: the six ramps of a colour card
// end to end, every other one turned round so light meets light and
// dark meets dark. Blended in OKLab, two samples a cell by half blocks.
//
// the command and its verbs are in tool.go; it is on pzm.
package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/janearc/libtheme-css/spaces/ok"
	"github.com/janearc/libtheme-css/spaces/srgb"
	"golang.org/x/term"
)

// runs are the card's ramps: six of them, five stops each, from the
// darkest to the palest. The last is the grey one.
var runs = [][]string{
	{"1b1040", "5a2071", "b8446b", "ff8f4a", "ffd08a"},
	{"04202a", "0e5a66", "2de2e6", "9df7f2", "e8fffd"},
	{"2a0b44", "6a2f8f", "c44fb0", "ff8ad0", "ffd6ee"},
	{"3a1a06", "8a4410", "e08a22", "f5c542", "fff0b8"},
	{"07142a", "16386e", "3f7ad6", "8fc0f2", "dcebff"},
	{"140f14", "3a3038", "6e6068", "aaa0a6", "eae4e8"},
}

// lamps is one colour as the three bytes a terminal wants.
type lamps [3]uint8

// size is the columns and rows drawn when the output is not a terminal.
var size = [2]int{177, 12}

// trim is how many stops each ramp gives up at both ends, and grey is
// whether the grey ramp is in the loop: the ends are the palest and
// darkest, and the vivid middles are what the card shows off.
var (
	trim = 1
	grey = false
)

// lightness and chroma scale every stop in OKLCH, hue held: 1 is the
// card as it was drawn.
var lightness, chroma = 1.0, 1.0

// samples is how many colours the loop is worked out at, once: enough
// that no strip is wide enough to show a step between two.
const samples = 8192

// turn draws the loop across the whole pane and turns it past, so many
// frames a second, until it is told to stop.
func turn(round wheel, set knobs, fps int) {
	out := os.Stdout
	fmt.Fprint(out, "\x1b[?1049h\x1b[?25l")
	defer fmt.Fprint(out, "\x1b[0m\x1b[?25h\x1b[?1049l")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	began := time.Now()
	tick := time.NewTicker(time.Second / time.Duration(fps))
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-tick.C:
			turned := now.Sub(began).Seconds() * set.speed
			fmt.Fprint(out, frame(round, set.span, turned))
		}
	}
}

// loop is every stop in the order the loop runs: each ramp as it is,
// then the next turned round, so the sixth ends dark where the first
// begins.
func loop() []ok.OKLab {
	stops, kept := []ok.OKLab{}, runs
	if !grey {
		kept = runs[:len(runs)-1]
	}
	for index, run := range kept {
		run = run[trim : len(run)-trim]
		for step := range run {
			if index%2 == 1 {
				step = len(run) - 1 - step
			}
			stop := ok.FromSwatch(srgb.MustHex(run[step]).Swatch())
			stops = append(stops, moved(stop))
		}
	}
	return stops
}

// moved is a stop with lightness and chroma applied, in OKLCH, its hue
// held and its lightness kept between black and white.
func moved(stop ok.OKLab) ok.OKLab {
	polar := stop.Polar()
	polar.L = min(max(polar.L*lightness, 0), 1)
	polar.C = max(polar.C*chroma, 0)
	return polar.Rect()
}

// sampled is the closed loop as count colours, a catmull-rom curve
// through the stops, so a ramp bends through each stop rather than
// turning a corner at it.
//
// The colours are spaced evenly by how far the colour moves in OKLab,
// not by stop, so a join between two ramps, which moves furthest, gets
// the room it needs and nothing rushes.
func sampled(stops []ok.OKLab, count int) wheel {
	path := walked(stops, 2048)
	travelled := make([]float64, len(path))
	for index := 1; index < len(path); index++ {
		travelled[index] = travelled[index-1] +
			ok.Distance(path[index-1], path[index])
	}
	total := travelled[len(path)-1]
	table, step := make([]lamps, count), 0
	for index := range table {
		goal := float64(index) / float64(count) * total
		for step < len(path)-2 && travelled[step+1] < goal {
			step++
		}
		shown, _ := srgb.FromSwatch(
			eased(path[step].Polar()).Rect().Swatch())
		red, green, blue := shown.Bytes()
		table[index] = lamps{red, green, blue}
	}
	return wheel{table, total}
}

// wheel is the loop as it is drawn: its colours, evenly spaced, and how
// far the colour moves once round, in OKLab.
type wheel struct {
	colours []lamps
	length  float64
}

// fitted is the span the eye chooses for a pane this wide: the loop
// stretched until no character differs from the next by more than a
// quarter of what an eye can see, so no step shows.
func (round wheel) fitted(columns int) float64 {
	return round.length / (float64(columns) * ok.Eye / 4)
}

// eased is a colour brought inside what srgb can show by its chroma alone,
// softly: below nine tenths of the most this lightness and hue can have,
// it is left as it is, and above, it bends toward the most and never
// reaches it.
//
// Fitted hard, a ramp pushed past the edge runs into it and slides
// along it, and the turn shows as a crease.
func eased(colour ok.OKLCH) ok.OKLCH {
	edge, _ := ok.Fit(ok.OKLCH{L: colour.L, C: 0.5, H: colour.H}, srgb.In)
	most := edge.C
	knee := 0.9 * most
	if colour.C <= knee || most <= 0 {
		inside, _ := ok.Fit(colour, srgb.In)
		return inside
	}
	room := most - knee
	colour.C = knee + room*(1-math.Exp(-(colour.C-knee)/room))
	return colour
}

// walked is the curve round the loop, each segment in fine steps, back
// to where it began.
func walked(stops []ok.OKLab, fine int) []ok.OKLab {
	length, path := len(stops), []ok.OKLab{}
	for at := range length {
		for step := range fine {
			before, after := (at-1+length)%length, (at+1)%length
			path = append(path, curve(stops[before], stops[at],
				stops[after], stops[(at+2)%length],
				float64(step)/float64(fine)))
		}
	}
	return append(path, stops[0])
}

// curve is the uniform catmull-rom between from and to, a fraction along.
func curve(before, from, to, after ok.OKLab, along float64) ok.OKLab {
	blend := func(zero, one, two, three float64) float64 {
		return 0.5 * (2*one + (two-zero)*along +
			(2*zero-5*one+4*two-three)*along*along +
			(3*one-zero-3*two+three)*along*along*along)
	}
	return ok.OKLab{
		L: blend(before.L, from.L, to.L, after.L),
		A: blend(before.A, from.A, to.A, after.A),
		B: blend(before.B, from.B, to.B, after.B),
	}
}

// frame is the whole pane at a moment: one row of half blocks, the left
// half in the ink and the right in the ground, repeated down the pane.
func frame(round wheel, span, turned float64) string {
	columns, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		columns, rows = size[0], size[1]
	}
	if span <= 0 {
		span = round.fitted(columns)
	}
	row := strings.Builder{}
	samples := float64(columns * 2)
	for column := range columns {
		left := at(round.colours, float64(column*2)/samples/span+turned)
		right := at(round.colours,
			float64(column*2+1)/samples/span+turned)
		fmt.Fprintf(&row, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▌",
			left[0], left[1], left[2], right[0], right[1], right[2])
	}
	line := row.String()
	whole := strings.Builder{}
	if err == nil {
		whole.WriteString("\x1b[H")
	}
	for index := range rows {
		whole.WriteString(line)
		if index < rows-1 {
			whole.WriteString("\x1b[0m\r\n")
		}
	}
	whole.WriteString("\x1b[0m")
	return whole.String()
}

// styled is one row the width of size, as tmux writes styles, for a
// status line: half blocks, the left half the fg and the right the bg.
// Words, when there are some, sit from the second column on, each letter
// on the strip's colour under it, in an ink that reads there.
func styled(round wheel, span, turned float64, words string,
	ink lamps) string {
	columns := size[0]
	if span <= 0 {
		span = round.fitted(columns)
	}
	letters := []rune(words)
	row := strings.Builder{}
	samples := float64(columns * 2)
	for column := range columns {
		left := at(round.colours, float64(column*2)/samples/span+turned)
		right := at(round.colours,
			float64(column*2+1)/samples/span+turned)
		if column >= 1 && column-1 < len(letters) {
			under := mixed(left, right)
			row.WriteString(lettered(letters[column-1],
				inkOn(ink, under), under))
			continue
		}
		fmt.Fprintf(&row, "#[fg=#%02x%02x%02x,bg=#%02x%02x%02x]▌",
			left[0], left[1], left[2], right[0], right[1], right[2])
	}
	return row.String()
}

// lettered is one letter as tmux writes it, in an ink on a ground; a
// hash doubled, so tmux draws it and does not read a style.
func lettered(letter rune, ink, ground lamps) string {
	text := string(letter)
	if letter == '#' {
		text = "##"
	}
	return fmt.Sprintf("#[fg=#%02x%02x%02x,bg=#%02x%02x%02x]%s",
		ink[0], ink[1], ink[2], ground[0], ground[1], ground[2], text)
}

// mixed is the colour halfway between two, a lamp at a time: what a
// whole cell shows where a letter takes the place of two half blocks.
func mixed(one, other lamps) lamps {
	return lamps{halfway(one[0], other[0]), halfway(one[1], other[1]),
		halfway(one[2], other[2])}
}

// halfway is the whole number halfway between two bytes, rounded down,
// worked in bytes: half of each, and the one both give up when both are
// odd.
func halfway(one, other uint8) uint8 {
	return one/2 + other/2 + one&other&1
}

// at is the colour a place round the loop has, the place taken modulo
// one turn.
func at(table []lamps, place float64) lamps {
	place -= math.Floor(place)
	return table[int(place*float64(len(table)))%len(table)]
}

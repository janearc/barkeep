package main

import (
	"strings"
	"testing"
	"time"
)

// blankStrip is a strip of cells, rows by columns, as a ramp draws it.
func blankStrip(rows, columns int) [][]cell {
	cells := make([][]cell, rows)
	for row := range cells {
		cells[row] = make([]cell, columns)
		for column := range cells[row] {
			cells[row][column] = cell{glyph: halfBlock,
				ink: lamps{200, 40, 40}, ground: lamps{20, 20, 80}}
		}
	}
	return cells
}

// every wide cell is followed by an under cell, and none sits at the
// strip's last column, whatever the moment
func TestHoundsWideCellsAreAlwaysPaired(t *testing.T) {
	field := houndsFor(7, lamps{209, 198, 228}, steady(colourways[0]),
		time.Hour)
	for _, size := range [][2]int{{12, 5}, {40, 5}, {7, 2}, {30, 11}} {
		for step := range 200 {
			cells := blankStrip(size[0], size[1])
			field.over(cells, time.Duration(step)*97*time.Millisecond)
			for row := range cells {
				for column, each := range cells[row] {
					if each.wide && (column+1 >= size[1] ||
						!cells[row][column+1].under) {
						t.Fatalf("%v at %d: wide cell at %d,%d has no under cell",
							size, step, row, column)
					}
					if each.under && (column == 0 || !cells[row][column-1].wide) {
						t.Fatalf("%v at %d: under cell at %d,%d follows no wide cell",
							size, step, row, column)
					}
				}
			}
		}
	}
}

// a hound never covers a reading
func TestHoundsLeaveReadingsAlone(t *testing.T) {
	field := houndsFor(11, lamps{209, 198, 228}, steady(colourways[0]),
		time.Hour)
	for step := range 300 {
		cells := blankStrip(12, 5)
		for column := range cells[4] {
			cells[4][column] = cell{glyph: 'S', ink: lamps{255, 255, 255},
				ground: lamps{0, 0, 0}, words: true}
		}
		field.over(cells, time.Duration(step)*83*time.Millisecond)
		for column, each := range cells[4] {
			if each.glyph != 'S' || !each.words {
				t.Fatalf("at %d: the reading at column %d became %q", step,
					column, each.glyph)
			}
		}
	}
}

// the field moves: some hound is on another row a second later, and the
// first lane goes down while the second goes up
func TestHoundsMoveBothWays(t *testing.T) {
	pack := pack(3, 24, 2)
	moved := false
	for _, each := range pack {
		before, _ := each.at(24, 0)
		after, _ := each.at(24, 400*time.Millisecond)
		if before != after {
			moved = true
		}
		if each.lane == 0 && !each.down || each.lane == 1 && each.down {
			t.Fatalf("lane %d goes the wrong way", each.lane)
		}
	}
	if !moved {
		t.Fatal("no hound moved in 400 ms")
	}
}

// a strip five wide holds two lanes; one too narrow for a glyph, none
func TestHoundsLanes(t *testing.T) {
	for columns, want := range map[int]int{1: 0, 2: 1, 4: 1, 5: 2, 8: 3} {
		if got := lanes(blankStrip(3, columns)); got != want {
			t.Errorf("%d columns: %d lanes, want %d", columns, got, want)
		}
	}
}

// glyphsIn is how many glyphs a row writes, its colour codes left out.
func glyphsIn(row string) int {
	count, escaped := 0, false
	for _, letter := range row {
		switch {
		case letter == '\x1b':
			escaped = true
		case escaped:
			escaped = letter != 'm'
		default:
			count++
		}
	}
	return count
}

// the encoder writes a wide glyph once and nothing for the cell it
// covers: four glyphs for five cells, one of them two cells wide, so a
// row is as wide on the terminal as the strip
func TestEncodedSkipsUnderCells(t *testing.T) {
	cells := blankStrip(1, 5)[0]
	run([][]cell{cells}, 0, 0, '🐶')
	row := encoded(cells)
	if strings.Count(row, "🐶") != 1 {
		t.Fatalf("the hound is written %d times", strings.Count(row, "🐶"))
	}
	if got := glyphsIn(row); got != 4 {
		t.Fatalf("%d glyphs written for five cells with a hound, want 4", got)
	}
}

// anti-test: a row that wrote its under cell would be a cell too wide on
// the terminal, and the count must see it
func TestEncodedUnderCellAntiTest(t *testing.T) {
	cells := blankStrip(1, 5)[0]
	run([][]cell{cells}, 0, 0, '🐕')
	cells[1].under = false // as if the encoder ignored the flag
	if glyphsIn(encoded(cells)) == 4 {
		t.Fatal("a row with its under cell written still counts as right")
	}
}

package main

import (
	"testing"
	"time"
)

// A name and its reading share one row: the name from the left, the
// reading set to the right, and the row showing through between them.
func TestANameAndItsReadingShareOneRow(t *testing.T) {
	got := string(lettersOf(13, line{name: "cpu", reading: "1.7"}))
	if want := "cpu       1.7"; got != want {
		t.Errorf("the row reads %q; want %q", got, want)
	}
}

// and the widest pair the strip shows, a name of five and a reading of
// five, still has its gap at the strip's thirteen cells.
func TestTheWidestPairFitsTheStrip(t *testing.T) {
	got := string(lettersOf(13, line{name: "trash", reading: "12.3g"}))
	if want := "trash   12.3g"; got != want {
		t.Errorf("the row reads %q; want %q", got, want)
	}
}

// On a row too narrow for both, the reading is whole and the name is
// what gives way, with the gap kept; and a row narrower than the reading
// shows as much of the reading as there is room for, and no name.
func TestANarrowRowKeepsTheReadingWhole(t *testing.T) {
	for _, each := range []struct {
		columns int
		want    string
	}{{7, "mp (+1)"}, {5, " (+1)"}, {4, "(+1)"}, {3, "(+1"}} {
		got := string(lettersOf(each.columns,
			line{name: "mpres", reading: "(+1)"}))
		if got != each.want {
			t.Errorf("at %d cells the row reads %q; want %q",
				each.columns, got, each.want)
		}
	}
}

// A rule is dashes the whole width, whatever else the line holds, and a
// line with nothing on it is a row that shows through everywhere.
func TestARuleIsTheWidthAndAnEmptyLineIsClear(t *testing.T) {
	if got := string(lettersOf(6, line{rule: true, name: "x"})); got != "------" {
		t.Errorf("a rule reads %q", got)
	}
	if got := string(lettersOf(6, line{})); got != "      " {
		t.Errorf("an empty line reads %q", got)
	}
}

// The cells a row is drawn with hold its letters where lettersOf put
// them, marked as words, and half blocks everywhere else.
func TestARowsCellsHoldItsLettersAndHalfBlocksBetween(t *testing.T) {
	cells := rowOf(13, lamps{10, 10, 10}, lamps{20, 20, 20},
		line{name: "gpu", reading: "5"}, lamps{200, 200, 200})
	for column, each := range cells {
		letter, isWord := map[int]rune{0: 'g', 1: 'p', 2: 'u', 12: '5'}[column]
		if isWord != each.words || (isWord && each.glyph != letter) ||
			(!isWord && each.glyph != halfBlock) {
			t.Errorf("column %d holds %q, words %v", column, each.glyph,
				each.words)
		}
	}
}

// The strip has one row for each reading, in its order, and the rule
// under them: eleven rows where there were twenty-one.
func TestEveryReadingHasOneRowAndTheRuleIsLast(t *testing.T) {
	lines := status(time.Now())
	names := []string{"cpu", "gpu", "mem", "mpres", "swap", "disk",
		"trash", "rndr", "f/m", "eta"}
	if len(lines) != len(names)+1 {
		t.Fatalf("the strip has %d rows; want %d", len(lines), len(names)+1)
	}
	for at, name := range names {
		if lines[at].name != name || lines[at].reading == "" ||
			lines[at].rule {
			t.Errorf("row %d is %+v; want %s with a reading", at,
				lines[at], name)
		}
	}
	if last := lines[len(lines)-1]; !last.rule {
		t.Errorf("the last row is %+v; want the rule", last)
	}
}

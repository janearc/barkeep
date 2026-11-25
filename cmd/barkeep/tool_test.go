package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// escapes are a terminal's colour and cursor codes.
var escapes = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

// plain is what a terminal would show of some output: its codes gone.
func plain(shown string) string {
	return escapes.ReplaceAllString(shown, "")
}

// said runs the command with some words and gives what it printed, and
// the code it left.
func said(words ...string) (string, int) {
	var out bytes.Buffer
	code := tool().Run(words, &out, &out)
	return out.String(), code
}

// The strip, printed once: each reading's name down the left, one to a
// row, the rule under them, and the loop alone under the rule.
func Example_strip() {
	shown, code := said("strip", "--once", "--columns", "15", "--rows", "14")
	for _, row := range strings.Split(strings.TrimSpace(plain(shown)), "\n") {
		names := strings.FieldsFunc(row, func(cell rune) bool {
			return cell == halfBlock
		})
		fmt.Println(strings.Join(names[:min(1, len(names))], ""))
	}
	fmt.Println(code)
	// Output:
	// cpu
	// gpu
	// mem
	// mpres
	// swap
	// disk
	// trash
	// gocache
	// job
	// rndr
	// f/m
	// eta
	// ---------------
	//
	// 0
}

// The loop, printed once: as many rows as asked for, each as many cells
// across. A frame's rows end as a raw terminal wants them, with a
// carriage return before the line feed.
func Example_ramps() {
	shown, code := said("ramps", "--once", "--columns", "8", "--rows", "2")
	rows := strings.Split(strings.TrimSpace(plain(shown)), "\r\n")
	fmt.Println(len(rows), utf8.RuneCountInString(rows[0]), code)
	// Output:
	// 2 8 0
}

// One row as tmux styles: a style for each cell, and the words on it
// from the second cell.
func Example_row() {
	shown, code := said("row", "--words", "hi", "--columns", "6")
	fmt.Println(strings.Count(shown, "#[fg=#"),
		strings.Contains(shown, "]h#[") && strings.Contains(shown, "]i#["),
		code)
	// Output:
	// 6 true 0
}

// The colourways, listed: one row for each, the card's five first, each
// row its name and as wide as asked.
func Example_colourways() {
	shown, code := said("colourways", "--columns", "12")
	rows := strings.Split(strings.TrimRight(plain(shown), "\n"), "\n")
	for _, row := range rows[:5] {
		fmt.Println(strings.TrimSpace(row), len([]rune(row)))
	}
	fmt.Println(len(rows) == len(colourways), code)
	// Output:
	// ember 12
	// lagoon 12
	// heather 12
	// honey 12
	// glacier 12
	// true 0
}

// drive, asked what it would do: it names the server and sets nothing.
func Example_drive() {
	shown, code := said("drive", "--dry-run", "/tmp/a-tmux-socket")
	fmt.Print(shown)
	fmt.Println(code)
	// Output:
	// would keep @ramp turning on the tmux server at /tmp/a-tmux-socket
	// 0
}

// Every verb is declared whole, and has the example its declaration
// names.
func TestTheToolIsDeclaredWhole(t *testing.T) {
	if err := tool().Examples(os.DirFS(".")); err != nil {
		t.Fatal(err)
	}
}

// A flag that is not what it should be is refused by name, with what
// was given, and nothing is drawn.
func TestAWrongFlagIsRefusedByName(t *testing.T) {
	for _, each := range []struct {
		words []string
		want  string
	}{
		{[]string{"strip", "--once", "--fps", "lots"},
			`--fps wants a whole number, not "lots"`},
		{[]string{"ramps", "--once", "--speed", "quick"},
			`--speed wants a number, not "quick"`},
		{[]string{"strip", "--once", "--lava", "long"},
			`--lava wants a length of time, as 45s or 2m, not "long"`},
		{[]string{"strip", "--once", "--turn", "soon"},
			`--turn wants a length of time, as 45s or 2m, not "soon"`},
		{[]string{"strip", "--once", "--colourway", "ember,nosuch"},
			`--colourway wants a colourway's name, as barkeep ` +
				`colourways lists them, not "nosuch"`},
		{[]string{"colourways", "--columns", "wide"},
			`--columns wants a whole number, not "wide"`},
		{[]string{"row", "--ink", "mauve"},
			`--ink wants a colour as hex, not "mauve"`},
		{[]string{"ramps", "--once", "--rows", "-3"},
			`--rows wants a whole number, not "-3"`},
		{[]string{"drive"}, "drive wants one tmux socket, and got 0"},
	} {
		shown, code := said(each.words...)
		if code == 0 || !strings.Contains(shown, each.want) ||
			strings.ContainsRune(shown, halfBlock) {
			t.Errorf("%v left %d and said %q; want a refusal saying %q",
				each.words, code, shown, each.want)
		}
	}
}

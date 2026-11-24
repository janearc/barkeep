package main

import (
	"fmt"
	"math"
	"strings"

	"charm.land/tone"
	libcolour "github.com/janearc/libreadme/colour"
	"github.com/janearc/libtheme-css/spaces/srgb"
	"github.com/janearc/puzzlemouth/pzm"
)

// A paint is one colour of the palette, by its name.
type paint struct {
	name   string
	colour lamps
}

// palette is CharmTone: every colour of it, in its own order, each by
// its name in small letters.
func palette() []paint {
	keys := tone.Keys()
	made := make([]paint, len(keys))
	for index, key := range keys {
		made[index].name = strings.ToLower(key.String())
		shown := srgb.MustHex(strings.TrimPrefix(key.Hex(), "#"))
		red, green, blue := shown.Bytes()
		made[index].colour = lamps{red, green, blue}
	}
	return made
}

// hexOf is a colour as six hex digits, as a ramp's stops are written.
func hexOf(colour lamps) string {
	return fmt.Sprintf("%02x%02x%02x", colour[0], colour[1], colour[2])
}

// waxOf is the colour of a lamp's wax, for its liquid. Of the paints
// that stand at least so far apart from the liquid, by the contrast
// libreadme measures, it is the one nearest the liquid in hue: the two
// then differ by how light they are, and not by colour.
//
// When no paint stands that far apart it is the one that stands
// furthest.
func waxOf(liquid lamps, paints []paint, least float64) lamps {
	wet := readerOf(liquid)
	nearest, furthest := liquid, liquid
	closest, most := math.Inf(1), 0.0
	for _, each := range paints {
		dry := readerOf(each.colour)
		apart := libcolour.Contrast(wet, dry)
		if apart > most {
			most, furthest = apart, each.colour
		}
		hue := libcolour.HueApart(wet, dry)
		if apart >= least && hue < closest {
			closest, nearest = hue, each.colour
		}
	}
	if math.IsInf(closest, 1) {
		return furthest
	}
	return nearest
}

// wayColumns is how wide a colourway is listed when no width is given,
// picked: the longest name and room beside it to see the wax.
const wayColumns = 32

// wayRow is one colourway as a row of cells so wide: its shades from
// the liquid at the left to the hottest wax at the right, and its name
// from the second cell, in the ink moved to read on each shade.
func wayRow(way colourway, columns int, ink lamps) []cell {
	cells := make([]cell, columns)
	name := []rune(way.name)
	for column := range cells {
		ground := way.shades[column*len(way.shades)/columns]
		cells[column] = cell{glyph: ' ', ink: ground, ground: ground}
		if at := column - 1; at >= 0 && at < len(name) {
			cells[column] = cell{glyph: name[at], words: true,
				ink: inkOn(ink, ground), ground: ground}
		}
	}
	return cells
}

// listed is the colourways verb: every colourway there is, one to a
// row, each by its name on its own shades.
func listed(call pzm.Call) error {
	made, err := setupOf(call)
	if err != nil {
		return err
	}
	read := reader{call: call}
	columns := read.whole("columns", wayColumns)
	if read.wrong != nil {
		return read.wrong
	}
	for _, way := range colourways {
		row := encoded(wayRow(way, columns, made.ink)) + "\x1b[0m"
		if _, err := fmt.Fprintln(call.Out(), row); err != nil {
			return err
		}
	}
	return nil
}

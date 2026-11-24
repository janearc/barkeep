package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	libcolour "github.com/janearc/libreadme/colour"
	"github.com/janearc/libtheme-css/spaces/srgb"
)

// Every colour of the palette has a name, in small letters, that no
// other colourway has: a name picks one lamp and never two.
func TestEveryColourwayHasItsOwnName(t *testing.T) {
	if len(palette()) == 0 {
		t.Fatal("the palette has no colours")
	}
	seen := map[string]bool{}
	for _, name := range wayNames() {
		if name == "" || name != strings.ToLower(name) {
			t.Errorf("%q is not a name in small letters", name)
		}
		if seen[name] {
			t.Errorf("two colourways are named %q", name)
		}
		seen[name] = true
	}
	if want := len(cardNames) + len(palette()); len(colourways) != want {
		t.Errorf("%d colourways, want the card's and the palette's %d",
			len(colourways), want)
	}
}

// A colour written as hex is the colour it was.
func TestHexOfIsTheColour(t *testing.T) {
	for _, colour := range []lamps{{0, 0, 0}, {255, 255, 255}, {7, 128, 250}} {
		red, green, blue := srgb.MustHex(hexOf(colour)).Bytes()
		if got := (lamps{red, green, blue}); got != colour {
			t.Errorf("%v as hex %q is %v", colour, hexOf(colour), got)
		}
	}
}

// the paints the wax is picked from in the tests: a liquid, a pale
// paint of its hue, a paler paint of another hue, and one too near the
// liquid to be told from it
var (
	deepBlue = lamps{20, 20, 80}
	paleBlue = lamps{180, 190, 255}
	yellow   = lamps{255, 255, 0}
	nearBlue = lamps{30, 30, 90}
	someWax  = []paint{{"near", nearBlue}, {"yellow", yellow}, {"pale", paleBlue}}
)

// The wax is the paint nearest the liquid in hue, of those far enough
// apart: the pale blue, and not the yellow that stands further apart,
// nor the blue too near to be told from the liquid.
func TestTheWaxIsTheNearestHueThatStandsApart(t *testing.T) {
	if got := waxOf(deepBlue, someWax, 3); got != paleBlue {
		t.Fatalf("the wax of a deep blue is %v, want the pale blue %v",
			got, paleBlue)
	}
}

// anti-test: the paints are such that the furthest apart is not the
// nearest in hue, so a rule that only took the most contrast would be
// seen
func TestTheWaxAntiTest(t *testing.T) {
	wet := readerOf(deepBlue)
	if libcolour.Contrast(wet, readerOf(yellow)) <=
		libcolour.Contrast(wet, readerOf(paleBlue)) {
		t.Fatal("the yellow does not stand further from the liquid " +
			"than the pale blue, so the test above shows nothing")
	}
	if libcolour.Contrast(wet, readerOf(nearBlue)) >= 3 {
		t.Fatal("the near blue stands apart, so the test above does " +
			"not show a near paint is passed over")
	}
}

// With no paint far enough apart, the wax is the one furthest.
func TestTheWaxIsTheFurthestWhenNoneStandsApart(t *testing.T) {
	if got := waxOf(deepBlue, someWax, 1000); got != yellow {
		t.Fatalf("with nothing far enough, the wax is %v, want %v",
			got, yellow)
	}
}

// Every colour of the palette has a wax from the palette that stands
// as far from it as the reader's chips must, or, for a colour in the
// middle that nothing in the palette stands that far from, the paint
// that stands furthest. No lamp's wax is its liquid.
func TestEveryPaletteLampHasAWaxThatStandsApart(t *testing.T) {
	paints := palette()
	least := reading.Contrast.ChipMin
	for _, liquid := range paints {
		wet := readerOf(liquid.colour)
		wax := waxOf(liquid.colour, paints, least)
		apart := libcolour.Contrast(wet, readerOf(wax))
		if wax == liquid.colour {
			t.Errorf("%s: its wax is its liquid", liquid.name)
		}
		if apart >= least {
			continue
		}
		for _, other := range paints {
			if libcolour.Contrast(wet, readerOf(other.colour)) > apart {
				t.Errorf("%s: its wax stands %.2f apart, under %.2f, "+
					"and %s stands further", liquid.name, apart, least,
					other.name)
			}
		}
	}
}

// A colourway's row begins on its liquid and ends on its hottest wax,
// with its name from the second cell, each letter in an ink that reads
// on the shade under it.
func TestAColourwaysRowIsItsShadesUnderItsName(t *testing.T) {
	way, ink := colourways[0], lamps{209, 198, 228}
	cells := wayRow(way, 20, ink)
	last := way.shades[19*len(way.shades)/20]
	if cells[0].ground != way.shades[0] || cells[19].ground != last {
		t.Fatalf("the row runs from %v to %v, want %v to %v",
			cells[0].ground, cells[19].ground, way.shades[0], last)
	}
	if cells[0].words || cells[19].words {
		t.Fatal("the row's ends are letters, and should be colour alone")
	}
	for at, letter := range way.name {
		each := cells[at+1]
		if each.glyph != letter || !each.words {
			t.Fatalf("cell %d is %q, want the name's %q", at+1,
				each.glyph, letter)
		}
		if each.ink != inkOn(ink, each.ground) {
			t.Fatalf("the letter %q is not in the ink that reads on "+
				"its shade", letter)
		}
	}
}

// A row too narrow for the name holds what of it fits, and no more
// cells than asked.
func TestAColourwaysRowIsNoWiderThanAsked(t *testing.T) {
	cells := wayRow(colourways[0], 3, lamps{})
	if len(cells) != 3 || cells[1].glyph != 'e' || cells[2].glyph != 'm' {
		t.Fatalf("three cells of ember are %d cells, %q and %q",
			len(cells), cells[1].glyph, cells[2].glyph)
	}
}

// The colourways of some names are those, in the order asked.
func TestNamedKeepsTheOrderAsked(t *testing.T) {
	last := colourways[len(colourways)-1].name
	found, err := named([]string{last, "ember", "honey"})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{found[0].name, found[1].name, found[2].name}
	if want := []string{last, "ember", "honey"}; !slices.Equal(got, want) {
		t.Fatalf("named gave %v, want %v", got, want)
	}
}

// A name that is no colourway's is refused, and the refusal says it.
func TestNamedRefusesANameItDoesNotKnow(t *testing.T) {
	_, err := named([]string{"ember", "no such lamp"})
	if err == nil || !strings.Contains(err.Error(), `"no such lamp"`) {
		t.Fatalf("the refusal is %v, and does not name the name", err)
	}
}

// sameShades says whether two lamps' shades are the same shades.
func sameShades(got, want []lamps) bool {
	return slices.Equal(got, want)
}

// A lamp turning for ever is each colourway's own shades for most of
// its time, is half way to the next half way through the turning, and
// after the last is the first again.
func TestALampTurnsRoundAndRound(t *testing.T) {
	ways := colourways[:3]
	each, turning := 10*time.Second, 4*time.Second
	for since, way := range map[time.Duration]int{0: 0,
		6 * time.Second: 0, 10 * time.Second: 1, 20 * time.Second: 2,
		30 * time.Second: 0, 46 * time.Second: 1} {
		got := turningAt(ways, each, turning, since)
		if !sameShades(got, ways[way].shades) {
			t.Errorf("%v in, the lamp is not colourway %d", since, way)
		}
	}
	// half way through the last one's turning it is half way to the
	// first
	half := turningAt(ways, each, turning, 28*time.Second)
	for index := range half {
		want := between(ways[2].shades[index], ways[0].shades[index], 0.5)
		if half[index] != want {
			t.Fatalf("turning back to the first, shade %d is %v, want %v",
				index, half[index], want)
		}
	}
}

// withPlay sets the colourways in play and how often lamps turn for one
// test, and puts both back after it.
func withPlay(t *testing.T, ways []colourway, every time.Duration) {
	t.Helper()
	wasPlay, wasEvery := inPlay, turnEvery
	t.Cleanup(func() { inPlay, turnEvery = wasPlay, wasEvery })
	inPlay, turnEvery = ways, every
}

// With no turning, each lamp lit is the next colourway in play, round
// and round, and stays it however long it plays.
func TestALampThatDoesNotTurnStaysItsColourway(t *testing.T) {
	withPlay(t, colourways[2:5], 0)
	for lit, way := range []int{2, 3, 4, 2} {
		for _, since := range []time.Duration{0, time.Minute, 9 * time.Hour} {
			if !sameShades(shadedFor(lit)(since), colourways[way].shades) {
				t.Errorf("lamp %d, %v in, is not colourway %d", lit,
					since, way)
			}
		}
	}
}

// With turning, a lamp begins as its own colourway and is the next in
// play when its time is up; the last in play turns to the first.
func TestALampThatTurnsBeginsAsItsOwnColourway(t *testing.T) {
	withPlay(t, colourways[2:5], time.Minute)
	for lit, ways := range [][2]int{{2, 3}, {3, 4}, {4, 2}} {
		lamp := shadedFor(lit)
		if !sameShades(lamp(0), colourways[ways[0]].shades) {
			t.Errorf("lamp %d does not begin as colourway %d", lit, ways[0])
		}
		if !sameShades(lamp(time.Minute), colourways[ways[1]].shades) {
			t.Errorf("lamp %d, a minute in, is not colourway %d", lit,
				ways[1])
		}
	}
}

// A lamp that turns takes no longer over a turn than turnTakes, and no
// more than half the time it stays a colourway.
func TestATurnIsNoLongerThanHalfALampsTime(t *testing.T) {
	withPlay(t, colourways[:2], 10*time.Second)
	lamp := shadedFor(0)
	if !sameShades(lamp(5*time.Second), colourways[0].shades) {
		t.Error("half way through its time the lamp has begun to turn")
	}
	if sameShades(lamp(8*time.Second), colourways[0].shades) {
		t.Error("in the last half of its time the lamp has not begun to turn")
	}
}

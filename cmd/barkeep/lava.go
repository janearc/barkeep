package main

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/janearc/libtheme-css/spaces/ok"
	"github.com/janearc/libtheme-css/spaces/srgb"
)

// The lava lamp. Blobs of wax rise and sink through a liquid, slowly,
// meeting and parting, with a pool of wax at the foot and a smaller one
// under the cap. It comes up over the ramp, plays for as long as it is
// set to, and goes back down to the ramp.
//
// the strip is drawn two colours a cell, so the lamp is as many dots
// across as the strip has columns and twice as many down as it has
// rows.
//
// A dot's colour is how much wax is near it: each blob adds its size
// squared over its distance squared, and where that sum crosses one is
// a blob's edge. The edge is soft, so a blob glows.
const (
	// picked: enough to meet and part in a strip five dots wide
	lavaBlobs = 6
	// picked: the lamp comes up over the ramp this long, and goes back
	// down this long
	lavaRises = 2500 * time.Millisecond
	lavaSinks = 2500 * time.Millisecond
	// picked: frames a second; wax is slow
	lavaRate = 10

	// picked: a blob takes this long, in seconds, to go from the foot to
	// the cap and back: the quickest and the slowest
	lavaQuick, lavaSlow = 22.0, 46.0
	// picked: and this long to drift across the strip and back
	lavaDriftQuick, lavaDriftSlow = 31.0, 67.0
	// picked: a blob's radius, in dots: the smallest and the largest
	lavaSmall, lavaBig = 1.3, 2.6
	// picked: how far from the middle a blob goes, as a share of the
	// height, nearly to both ends; and how far across, as a share of
	// the width
	lavaTravel = 0.46
	lavaDrift  = 0.3
	// picked: the wax at the foot and under the cap, each falling away
	// over this many dots
	lavaFoot = 2.2
	lavaCap  = 0.9
	lavaDeep = 3.0
	// picked: the sum of wax where the liquid ends and where the wax is
	// whole; between them is a blob's glowing edge
	lavaThin, lavaThick = 0.55, 1.7
	lavaShadeCount      = 256 // picked: the shades between liquid and wax
)

// lavaLasts is how long a lava lamp plays, the -lava flag.
var lavaLasts = 45 * time.Second

// A colourway is a lamp's colours by a name: its shades from the liquid
// to the hottest wax, evenly, blended in OKLab and brought inside what
// srgb can show.
type colourway struct {
	name   string
	shades []lamps
}

// cardNames are the names, picked, of the lamps made from the card's
// ramps, in the ramps' order: one for each of the five coloured ramps
// in runs. The grey ramp makes no lamp.
var cardNames = []string{"ember", "lagoon", "heather", "honey", "glacier"}

// colourways are the lamps there are: the card's five, then one for
// every colour of the palette.
var colourways = ways()

// ways makes the colourways. A card lamp is its ramp without the
// darkest stop, so the liquid is the ramp's deep colour and not its
// near-black. A palette lamp is named for its liquid, and its wax is
// the palette's own colour that waxOf picks for it.
func ways() []colourway {
	made := make([]colourway, 0, len(cardNames))
	for index, name := range cardNames {
		made = append(made, colourway{name,
			shades(runs[index][1:], lavaShadeCount)})
	}
	paints := palette()
	for _, liquid := range paints {
		wax := waxOf(liquid.colour, paints, reading.Contrast.ChipMin)
		stops := []string{hexOf(liquid.colour), hexOf(wax)}
		made = append(made, colourway{liquid.name,
			shades(stops, lavaShadeCount)})
	}
	return made
}

// cardWays are the card's own lamps, the first of the colourways.
func cardWays() []colourway {
	return colourways[:len(cardNames)]
}

// wayNames are the colourways' names, in their order.
func wayNames() []string {
	names := make([]string, len(colourways))
	for index, each := range colourways {
		names[index] = each.name
	}
	return names
}

// inPlay are the colourways the lamps are lit in, in turn: every one
// there is, or the ones --colourway names.
var inPlay = colourways

// named are the colourways of some names, in the order given; and, for
// a name that is no colourway's, a refusal that says which.
func named(names []string) ([]colourway, error) {
	found := make([]colourway, 0, len(names))
	for _, name := range names {
		at := slices.IndexFunc(colourways, func(each colourway) bool {
			return each.name == name
		})
		if at < 0 {
			return nil, fmt.Errorf("--colourway wants a "+
				"colourway's name, as barkeep colourways "+
				"lists them, not %q", name)
		}
		found = append(found, colourways[at])
	}
	return found, nil
}

// wayFor is the colourway of the lamp that is so many lamps after the
// first: the next of those in play, round and round.
func wayFor(lit int) colourway {
	return inPlay[lit%len(inPlay)]
}

// turnEvery is how long a lamp stays one colourway before it turns to
// the next of those in play, the --turn flag. Nought, a lamp stays the
// colourway it was lit in.
var turnEvery time.Duration

// turnTakes is the longest a lamp takes to turn from one colourway to
// the next, picked: slow enough that the turn is not seen to start. A
// lamp that turns more often than twice this takes half its time.
const turnTakes = 20 * time.Second

// shadedFor is the shades of the lamp that is so many lamps after the
// first, at each moment of its playing: its colourway's own, or, when
// lamps turn, each colourway in play in turn from its own, for ever.
func shadedFor(lit int) func(time.Duration) []lamps {
	if turnEvery <= 0 {
		return steady(wayFor(lit))
	}
	first := lit % len(inPlay)
	round := append(slices.Clone(inPlay[first:]), inPlay[:first]...)
	turning := min(turnTakes, turnEvery/2)
	return func(since time.Duration) []lamps {
		return turningAt(round, turnEvery, turning, since)
	}
}

// turningAt is the shades of a lamp turning through colourways for
// ever, so long into its playing: each one's own for so long, the last
// of that time spent turning into the next, and after the last the
// first again.
func turningAt(ways []colourway, each, turning,
	since time.Duration) []lamps {
	at := int(since/each) % len(ways)
	into := since % each
	if into <= each-turning {
		return ways[at].shades
	}
	share := smooth(float64(into-(each-turning)) / float64(turning))
	return blended(ways[at].shades, ways[(at+1)%len(ways)].shades, share)
}

// blended is two lamps' shades, a share of the way from one to the
// other, shade by shade.
func blended(from, to []lamps, share float64) []lamps {
	turned := make([]lamps, len(from))
	for index := range turned {
		turned[index] = between(from[index], to[index], share)
	}
	return turned
}

// shades is a run of stops as so many colours, from the first stop to
// the last, each blended in OKLab between the two stops it lies between.
func shades(stops []string, count int) []lamps {
	marks := make([]ok.OKLab, len(stops))
	for index, stop := range stops {
		marks[index] = ok.FromSwatch(srgb.MustHex(stop).Swatch())
	}
	table := make([]lamps, count)
	for index := range table {
		along := float64(index) / float64(count-1) *
			float64(len(marks)-1)
		from := min(int(along), len(marks)-2)
		share := along - float64(from)
		one, other := marks[from], marks[from+1]
		blend := ok.OKLab{L: one.L + (other.L-one.L)*share,
			A: one.A + (other.A-one.A)*share,
			B: one.B + (other.B-one.B)*share}
		shown, _ := srgb.FromSwatch(
			eased(blend.Polar()).Rect().Swatch())
		red, green, blue := shown.Bytes()
		table[index] = lamps{red, green, blue}
	}
	return table
}

// A blob is one blob of wax: how big it is, and how it goes up and down
// and across, each a slow swing with a period and a start of its own.
type blob struct {
	size                float64
	every, from         float64
	driftEvery, driftAt float64
}

// blobs are a lamp's blobs, picked by a seed.
func blobs(seed uint64) []blob {
	made := make([]blob, lavaBlobs)
	for index := range made {
		own := drop{seed: seed, row: uint64(index)}
		// within is a number between two, picked by the blob's own
		// share of the way, each asking for a different share
		within := func(low, high float64, which uint64) float64 {
			return low + (high-low)*own.share(which)
		}
		made[index] = blob{
			size:       within(lavaSmall, lavaBig, 1),
			every:      within(lavaQuick, lavaSlow, 2),
			from:       own.share(3),
			driftEvery: within(lavaDriftQuick, lavaDriftSlow, 4),
			driftAt:    own.share(5),
		}
	}
	return made
}

// at is where the blob's middle is at a time, in dots, in a lamp so
// wide and so high. A swing spends longest at its ends, as wax does at
// the foot and the cap.
func (wax blob) at(seconds, wide, high float64) (across, down float64) {
	swing := math.Sin(2 * math.Pi * (seconds/wax.every + wax.from))
	drift := math.Sin(2 * math.Pi * (seconds/wax.driftEvery + wax.driftAt))
	return wide * (0.5 + lavaDrift*drift), high * (0.5 + lavaTravel*swing)
}

// A lamp is the lava lamp at a moment: where each blob is, its size,
// how many times the size of the lamp its sizes were picked in it is,
// and its shades from liquid to wax.
type lamp struct {
	wide, high float64
	grown      float64
	middles    [][2]float64
	waxes      []blob
	shades     []lamps
}

// The lamp the sizes above were picked in, in dots: a side column five
// characters by forty. A blob's size, how
// soft its edge is and how deep the pools at the foot and the cap are
// were all picked there, in dots.
const lavaWide, lavaHigh = 5.0, 80.0

// grown is how many times larger than that lamp, along a side, a lamp so
// wide and so high is: by its area, so the wax covers the same share of
// a lamp of any size and shape: a blob is as large as its lamp.
func grown(wide, high float64) float64 {
	return math.Sqrt(wide * high / (lavaWide * lavaHigh))
}

// shade is the colour of the dot at a place: the wax near it, from the
// blobs and the two pools, as a shade between liquid and wax.
func (now lamp) shade(across, down float64) lamps {
	deep := lavaDeep * now.grown
	wax := lavaFoot*math.Exp(-(now.high-down)/deep) +
		lavaCap*math.Exp(-down/deep)
	for index, middle := range now.middles {
		apart := (across-middle[0])*(across-middle[0]) +
			(down-middle[1])*(down-middle[1])
		size := now.waxes[index].size * now.grown
		wax += size * size / (apart + 0.25*now.grown*now.grown)
	}
	thick := smooth((wax - lavaThin) / (lavaThick - lavaThin))
	return now.shades[int(thick*float64(len(now.shades)-1))]
}

// lava is the lava lamp as an effect, in the shades it is given for
// each moment, its blobs picked by a seed, with the ink the readings
// are written in before libreadme moves it.
//
// The colour under a reading changes as the wax goes by, so its ink is
// moved again each frame to read on what is under it now.
func lava(seed uint64, ink lamps,
	shaded func(time.Duration) []lamps) effect {
	return lavaShaded(seed, ink, lavaLasts, shaded)
}

// steady is the shades of a lamp that stays one colourway.
func steady(way colourway) func(time.Duration) []lamps {
	return func(time.Duration) []lamps { return way.shades }
}

// lavaThrough is one lava lamp that plays through colourways, each for
// so long, and is the next by the end of each: over the last of each
// colourway's time, turning, its shades go to the next one's.
//
// The blobs are the same blobs throughout and do not stop: a new lamp
// for each colourway would start its blobs again, which reads as an
// interruption.
func lavaThrough(seed uint64, ink lamps, ways []colourway,
	each, turning time.Duration) effect {
	turned := func(since time.Duration) []lamps {
		return shadesAt(ways, each, turning, since)
	}
	return lavaShaded(seed, ink, time.Duration(len(ways))*each, turned)
}

// shadesAt is the shades of a lamp playing through colourways, so long
// into its playing: a colourway's own, or, in the last of its time,
// between its own and the next one's. The last colourway has no next
// and stays itself.
func shadesAt(ways []colourway, each, turning, since time.Duration) []lamps {
	at := min(int(since/each), len(ways)-1)
	into := since - time.Duration(at)*each
	if at == len(ways)-1 || into <= each-turning {
		return ways[at].shades
	}
	share := smooth(float64(into-(each-turning)) / float64(turning))
	return blended(ways[at].shades, ways[at+1].shades, share)
}

// lavaShaded is a lava lamp that plays so long, in the shades it is
// given for each moment of its playing.
func lavaShaded(seed uint64, ink lamps, lasts time.Duration,
	shaded func(since time.Duration) []lamps) effect {
	waxes := blobs(seed)
	return effect{lasts: lasts, rate: lavaRate,
		over: func(cells [][]cell, since time.Duration) {
			if len(cells) == 0 || len(cells[0]) == 0 {
				return
			}
			now := lamp{wide: float64(len(cells[0])),
				high: float64(len(cells) * 2), waxes: waxes,
				shades: shaded(since)}
			now.grown = grown(now.wide, now.high)
			for _, wax := range waxes {
				across, down := wax.at(since.Seconds(),
					now.wide, now.high)
				now.middles = append(now.middles,
					[2]float64{across, down})
			}
			up := risen(since, lasts)
			for row := range cells {
				for column := range cells[row] {
					was := cells[row][column]
					cells[row][column] = now.over(was, row,
						column, up, ink)
				}
			}
		}}
}

// risen is how much of the lamp shows so long into its playing: up from
// nought as it begins and down to nought as it ends.
func risen(since, lasts time.Duration) float64 {
	in := smooth(float64(since) / float64(lavaRises))
	out := smooth(float64(lasts-since) / float64(lavaSinks))
	return min(in, out)
}

// over is one cell with so much of the lamp over it. A half block takes
// the lamp's two dots; a letter of the readings takes the colour
// between them as its ground, and its ink is moved to read there.
func (now lamp) over(was cell, row, column int, up float64, ink lamps) cell {
	across := float64(column) + 0.5
	upper := now.shade(across, float64(row*2)+0.5)
	lower := now.shade(across, float64(row*2)+1.5)
	if !was.words {
		was.ink = between(was.ink, upper, up)
		was.ground = between(was.ground, lower, up)
		return was
	}
	was.ground = between(was.ground, mixed(upper, lower), up)
	was.ink = inkOn(ink, was.ground)
	return was
}

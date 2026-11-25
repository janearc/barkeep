package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/janearc/libtheme-css/spaces/srgb"
	"github.com/janearc/puzzlemouth/pzm"
	"github.com/janearc/puzzlemouth/verb"
)

// the four stamps game build writes into a command's main package
var build, built, who, sessions string

// main runs the command.
func main() {
	os.Exit(tool().Run(os.Args[1:], os.Stdout, os.Stderr))
}

// tool is the command and its four verbs. The flags every verb answers
// to are the loop's: how it is coloured and how fast it turns.
func tool() *pzm.Tool {
	made := pzm.New("barkeep",
		"the machine's readings down the side of a terminal",
		pzm.Build(build), pzm.Built(built), pzm.Who(who),
		pzm.Sessions(sessions))
	made.Flags(
		pzm.Value("speed", verb.NoShort,
			"how much of the loop goes past in a second (0.02)"),
		pzm.Value("span", verb.NoShort,
			"pane lengths the loop takes; 0, the eye chooses"),
		pzm.Value("fps", verb.NoShort, "frames a second (30)"),
		pzm.Value("trim", verb.NoShort,
			"stops each ramp gives up at both ends (1)"),
		pzm.Switch("grey", verb.NoShort, "the grey ramp in the loop"),
		pzm.Value("lightness", verb.NoShort,
			"lightness times this: 0.8 dims, 1.2 lifts"),
		pzm.Value("chroma", verb.NoShort,
			"chroma times this: 0.5 calms, 1.3 sharpens"),
		pzm.Value("ink", verb.NoShort,
			"the words' ink, as hex, before it is moved to read"),
	)
	// every verb here draws and none writes a file; drive alone changes
	// something that outlasts it, an option on a tmux server.
	draws := verb.NewFacts(verb.WritesNothing, verb.LeavesAlone,
		verb.ThisMachine, verb.NoOne, verb.Light)
	sets := verb.NewFacts(verb.WritesTo("a tmux server's @ramp option"),
		verb.LeavesAlone, verb.ThisMachine, verb.NoOne, verb.Light)
	made.Verb("strip", 's', "the readings down this pane, over a lamp",
		verb.Page(stripPage), "Example_strip", draws, stripVerb,
		stripFlags()...)
	made.Verb("ramps", 'r', "the loop of colour across this pane",
		rampsPage, "Example_ramps", draws, ramps, sized()...)
	made.Verb("row", 'w', "one row of the loop, as tmux styles",
		rowPage, "Example_row", draws, row,
		pzm.Value("words", verb.NoShort, "words to draw on the row"),
		pzm.Value("columns", verb.NoShort, "the row's width (177)"))
	made.Verb("drive", 'd', "keep a tmux status row turning",
		drivePage, "Example_drive", sets, driven)
	made.Verb("colourways", 'c', "every lamp's colours, by name",
		waysPage, "Example_colourways", draws, listed,
		pzm.Value("columns", verb.NoShort, "each row's width (32)"))
	return made
}

// sized are the flags of a verb that can print one frame and leave.
func sized() []pzm.Flag {
	return []pzm.Flag{
		pzm.Switch("once", verb.NoShort,
			"one frame, printed and left, for looking at"),
		pzm.Value("columns", verb.NoShort,
			"the width, when not a terminal (177)"),
		pzm.Value("rows", verb.NoShort,
			"the height, when not a terminal (12)"),
	}
}

// stripFlags are the strip's own flags: what plays over the loop, and
// the one file it reads.
func stripFlags() []pzm.Flag {
	return append(sized(),
		pzm.Value("scramble", verb.NoShort,
			"how long the rain stays green before fading (400ms)"),
		pzm.Value("lava", verb.NoShort,
			"how long a lava lamp plays (45s)"),
		pzm.List("colourway", verb.NoShort,
			"the colourways the lamps are lit in, as ember,sakura"),
		pzm.Value("turn", verb.NoShort,
			"how long a lamp stays one colourway before the next"),
		pzm.Value("render", verb.NoShort,
			"a render watcher's progress file, including"+
				"an optional job name"),
		pzm.Switch("hounds", verb.NoShort,
			"open with the hounds over a lava lamp, not the rain"),
		pzm.Switch("show", verb.NoShort,
			"every effect in turn, each fading into the next"),
		pzm.Value("banner", verb.NoShort,
			"words in large letters alone, and no readings"),
		pzm.Value("font", verb.NoShort,
			"the figlet font a banner's letters use (alligator)"),
	)
}

// stripPage is the strip's page. It is not a constant because it lists
// the colourways there are.
var stripPage = `# strip

draws the machine's readings down this pane, one to a row, over a slow
loop of colour: the load, the gpu, memory and its trend, swap, the disk,
the Trash, the Go build cache, and a render's name and progress when
--render names its file. The Go cache follows GOCACHE or the default
user cache directory; its allocated disk space is measured in the
background every five minutes. A progress file may carry a job name on
its second line; older one-line files remain valid.

it is made for a narrow pane beside your work, thirteen cells or so. name
other terminals after the flags and it draws the same strip in each, from
this one process. it runs until it is stopped.

a lamp is lit in a colourway: its liquid and its wax. five are a colour
card's own ramps. the rest are CharmTone's colours, each a lamp named
for its liquid, with the wax libreadme's contrast picks for it: the
palette's colour nearest the liquid in hue that still stands apart.
--colourway names the ones in play, with commas between them, and
--turn has a lamp turn slowly from each to the next. a colour with no
partner far enough apart in the palette takes the one furthest from it.
barkeep colourways shows each in its colours.

run at a terminal, the strip takes keys: h has the hounds come and go,
c lights the next colourway and C the one before, and q or ctrl-c
leaves. the lamp keeps its wax where it was when a key changes it.

the colourways are:

` + wayList + `.

it reads the kernel, the I/O registry and the Trash's sizes. it starts no
other process and writes no file.`

const rampsPage = `# ramps

draws the loop of colour across the whole pane and turns it past until
it is stopped: the six ramps of a colour card end to end, every other
one turned round, so light meets light and dark meets dark.

--once prints one frame and leaves, for looking at.`

const waysPage = `# colourways

prints every colourway a lamp can be lit in, one to a row: its name,
written on its own colours, the liquid at the left and the hottest wax
at the right. a name here is what strip's --colourway takes.
`

const rowPage = `# row

prints one row of the loop as tmux styles, at this moment by the wall
clock, with --words drawn on it. it is for a status line that asks for
its row each time it redraws.`

const drivePage = `# drive

keeps one row of the loop turning on a tmux server, named by its socket:
every frame it sets the server's @ramp option to the row, with the words
of @bar on it, for a status line to show. it stops when it is told to,
and unsets @ramp as it goes.

with --dry-run it says which server it would drive and sets nothing.`

// wayList is the colourways' names, as a page lists them.
var wayList = strings.Join(wayNames(), ", ")

// A setup is what every verb draws with: the loop worked out, the knobs
// it turns by, the frames a second and the words' ink.
type setup struct {
	round wheel
	set   knobs
	fps   int
	ink   lamps
}

// setupOf reads the flags every verb answers to. It sets the loop's
// trim, grey, lightness and chroma before the loop is worked out, since
// the loop is made from them.
func setupOf(call pzm.Call) (setup, error) {
	read := reader{call: call}
	made := setup{fps: read.whole("fps", 30)}
	made.set.speed = read.number("speed", 0.02)
	made.set.span = read.number("span", 0)
	trim, grey = read.whole("trim", 1), call.On("grey")
	lightness = read.number("lightness", 1)
	chroma = read.number("chroma", 1)
	made.set.lightness, made.set.chroma = lightness, chroma
	hex := "d1c6e4"
	if call.Given("ink") {
		hex = call.Value("ink")
	}
	inked, err := srgb.FromHex(hex)
	if err != nil {
		read.fail("ink", "a colour as hex", hex)
	}
	if read.wrong != nil {
		return setup{}, read.wrong
	}
	made.ink[0], made.ink[1], made.ink[2] = inked.Bytes()
	made.round = sampled(loop(), samples)
	return made, nil
}

// A reader reads a call's flags as numbers and keeps the first one that
// was not what it should be, so a verb reads them all and asks once.
type reader struct {
	call  pzm.Call
	wrong error
}

// fail keeps what was wrong with a flag, if nothing was wrong before.
func (read *reader) fail(name, wants, given string) {
	if read.wrong == nil {
		read.wrong = fmt.Errorf("--%s wants %s, not %q", name, wants,
			given)
	}
}

// number is a flag's value as a number, or unset when it was not given.
func (read *reader) number(name string, unset float64) float64 {
	if !read.call.Given(name) {
		return unset
	}
	given := read.call.Value(name)
	value, err := strconv.ParseFloat(given, 64)
	if err != nil {
		read.fail(name, "a number", given)
	}
	return value
}

// whole is a flag's value as a whole number that is not negative, or
// unset when it was not given.
func (read *reader) whole(name string, unset int) int {
	if !read.call.Given(name) {
		return unset
	}
	given := read.call.Value(name)
	value, err := strconv.Atoi(given)
	if err != nil || value < 0 {
		read.fail(name, "a whole number", given)
	}
	return value
}

// lasting is a flag's value as a length of time, or unset when it was
// not given.
func (read *reader) lasting(name string, unset time.Duration) time.Duration {
	if !read.call.Given(name) {
		return unset
	}
	given := read.call.Value(name)
	value, err := time.ParseDuration(given)
	if err != nil {
		read.fail(name, "a length of time, as 45s or 2m", given)
	}
	return value
}

// sizeOfFrame reads --columns and --rows into the size a frame is drawn
// at when there is no terminal to ask.
func sizeOfFrame(call pzm.Call) error {
	read := reader{call: call}
	size[0] = read.whole("columns", size[0])
	size[1] = read.whole("rows", size[1])
	return read.wrong
}

// stripVerb is the strip verb: the readings down this pane and every
// pane named, until it is stopped; or, with --once, one frame printed.
func stripVerb(call pzm.Call) error {
	made, err := setupOf(call)
	if err != nil {
		return err
	}
	read := reader{call: call}
	scrambles = read.lasting("scramble", scrambles)
	lavaLasts = read.lasting("lava", lavaLasts)
	if read.wrong != nil {
		return read.wrong
	}
	turnEvery = read.lasting("turn", 0)
	if read.wrong != nil {
		return read.wrong
	}
	inPlay = colourways
	if call.Given("colourway") {
		asked, err := named(call.List("colourway"))
		if err != nil {
			return err
		}
		inPlay = asked
	}
	renderPath = call.Value("render")
	houndsFirst, showing = call.On("hounds"), call.On("show")
	if err := bannered(call); err != nil {
		return err
	}
	if !call.On("once") {
		return column(made.round, made.set, made.fps, made.ink,
			call.Args())
	}
	if err := sizeOfFrame(call); err != nil {
		return err
	}
	return printed(call.Out(), made, time.Now())
}

// bannered makes the banner's letters when --banner gives it words, in
// the font --font names.
func bannered(call pzm.Call) error {
	if !call.Given("banner") {
		return nil
	}
	font := "alligator"
	if call.Given("font") {
		font = call.Value("font")
	}
	lines, err := figlets(call.Value("banner"), font)
	banner = lines
	return err
}

// printed prints one frame of the strip as it is now, with nothing
// playing over it, a row to a line.
func printed(out io.Writer, made setup, now time.Time) error {
	shown := written{lines: status(now), ink: made.ink, read: now}
	at := moment{round: made.round, span: made.set.span}
	for _, row := range pictured(size, at, shown, overlay{}) {
		if _, err := fmt.Fprintln(out, row+"\x1b[0m"); err != nil {
			return err
		}
	}
	return nil
}

// ramps is the ramps verb: the loop across the pane until it is
// stopped; or, with --once, one frame printed.
func ramps(call pzm.Call) error {
	made, err := setupOf(call)
	if err != nil {
		return err
	}
	if err := sizeOfFrame(call); err != nil {
		return err
	}
	if !call.On("once") {
		turn(made.round, made.set, made.fps)
		return nil
	}
	_, err = fmt.Fprintln(call.Out(), frame(made.round, made.set.span, 0))
	return err
}

// row is the row verb: one row of the loop as tmux styles, turned as far
// as the wall clock says.
func row(call pzm.Call) error {
	made, err := setupOf(call)
	if err != nil {
		return err
	}
	read := reader{call: call}
	size[0] = read.whole("columns", size[0])
	if read.wrong != nil {
		return read.wrong
	}
	now := float64(time.Now().UnixNano()) / float64(time.Second)
	_, err = fmt.Fprint(call.Out(), styled(made.round, made.set.span,
		now*made.set.speed, call.Value("words"), made.ink))
	return err
}

// driven is the drive verb: one tmux server's @ramp kept turning, the
// server named by its socket.
func driven(call pzm.Call) error {
	if len(call.Args()) != 1 {
		return fmt.Errorf("drive wants one tmux socket, and got %d",
			len(call.Args()))
	}
	socket := call.Args()[0]
	if call.DryRun() {
		_, err := fmt.Fprintf(call.Out(),
			"would keep @ramp turning on the tmux server at %s\n",
			socket)
		return err
	}
	made, err := setupOf(call)
	if err != nil {
		return err
	}
	return drive(made.round, made.set, made.fps, socket, made.ink)
}

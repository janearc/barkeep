package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/sys/unix"
)

// The machine's readings, each cut to fit a narrow strip: the load, the
// GPU, memory and its trend, swap, the disk and the Trash. An unreadable
// one is "?", never nought.
//
// nothing here starts a process. The load, memory, swap and disk are
// asked of the kernel, and the GPU's share of IOKit.
//
// Memory's trend and the GPU's minute are worked from the readings this
// process has taken itself, so both are unknown for a while after it
// starts. The Trash is walked every few minutes, apart from the drawing.

// How far back a reading looks.
const (
	// the GPU's share is its mean over this long
	gpuOver = 60 * time.Second
	// the trend looks back this far, and wants a reading at least this
	// old
	trendBack = 600 * time.Second
	trendMin  = 300 * time.Second

	// A render's progress older than this is a render that has stopped
	// or finished, and its rows go back to a dash.
	renderStale = 120 * time.Second
)

// renderPath is the path to the render progress file (passed via -render),
// updated atomically via temp-file rename. When empty, render watching is
// disabled and render columns display "-".
//
// File format:
//
//	Line 1: "<done> <total> <frames_per_minute> <eta_seconds> <epoch>"
//	Line 2: optional job name
var renderPath string

// The kernel's memory pressure levels.
const (
	pressureNormal   = 1
	pressureWarn     = 2
	pressureCritical = 4
)

// A line is one row of words on the strip: a name, from the left, with
// its reading set to the right of the same row; or a rule, a row of
// dashes the strip's width.
type line struct {
	name, reading string
	rule          bool
}

// status is the readings as the strip's lines, top to bottom, each a
// name with its reading beside it on one row: cpu, gpu, mem, mpres,
// swap, disk, trash and Go build cache; then a job name, its progress,
// throughput and time left, a dash each when nothing renders.
//
// A rule is under them. Every reading always has its row, so nothing
// under one ever moves.
func status(now time.Time) []line {
	memory, trend := memoryLines(now)
	share, rate, left := render(renderPath, now)
	lines := []line{}
	for _, each := range [][2]string{
		{"cpu", load()},
		{"gpu", gpu(now)},
		{"mem", memory},
		{"mpres", trend},
		{"swap", swap()},
		{"disk", disk()},
		{"trash", trash(now)},
		{"gocache", goCache(now)},
		{"job", renderName(renderPath, now)},
		{"rndr", share},
		{"f/m", rate},
		{"eta", left},
	} {
		lines = append(lines, line{name: each[0], reading: each[1]})
	}
	return append(lines, line{rule: true})
}

// unknown is a reading that could not be taken: never nought.
const unknown = "?"

// load is the one-minute load average, to a tenth, cut and not rounded,
// as the script compares it.
func load() string {
	raw, err := unix.SysctlRaw("vm.loadavg")
	// three 32-bit fixed-point averages, four bytes of padding, and the
	// scale they are fixed by: struct loadavg in sys/resource.h
	if err != nil || len(raw) < 24 {
		return unknown
	}
	scale := binary.LittleEndian.Uint64(raw[16:24])
	if scale == 0 {
		return unknown
	}
	tenths := uint64(binary.LittleEndian.Uint32(raw[0:4])) * 10 / scale
	return fmt.Sprintf("%d.%d", tenths/10, tenths%10)
}

// gpu is the GPU's busy share, as a percentage, averaged over the last
// minute of this process's own readings of it: one reading is a moment,
// and a GPU's moments jump about. Unknown when the GPU will not say.
func gpu(now time.Time) string {
	busy, said := gpuBusy()
	if !said {
		return unknown
	}
	gpuSeen.add(now, busy)
	return strconv.Itoa(gpuSeen.mean())
}

// memoryLines is memory's free percentage as the kernel counts it, with
// a mark for the kernel's pressure level, since a level told by colour
// alone does not arrive: "!" at warn, "!!" at critical, "?" unknown.
//
// And its trend: the change since the reading about ten minutes ago, in
// brackets, "(0)" when it is steady and "(?)" when the history is too
// short to say.
func memoryLines(now time.Time) (memory, trend string) {
	free, err := unix.SysctlUint32("kern.memorystatus_level")
	if err != nil {
		return unknown, "(?)"
	}
	mark := "?"
	level, err := unix.SysctlUint32("kern.memorystatus_vm_pressure_level")
	if err == nil {
		mark = map[uint32]string{
			pressureNormal: "", pressureWarn: "!",
			pressureCritical: "!!",
		}[level]
		if level != pressureNormal && mark == "" {
			mark = "?"
		}
	}
	memorySeen.add(now, int(free))
	return fmt.Sprintf("%d%s", free, mark), trendOf(now, memorySeen)
}

// trendOf is how far memory's free percentage has moved since the
// earliest reading in the last ten minutes, when that reading is at
// least five minutes old: "(-3)" falling, "(+2)" easing, "(0)" steady,
// and "(?)" while the readings do not yet go back far enough.
func trendOf(now time.Time, seen history) string {
	moved, far := seen.moved(now, trendMin)
	if !far {
		return "(?)"
	}
	if moved == 0 {
		return "(0)"
	}
	return fmt.Sprintf("(%+d)", moved)
}

// swap is the swap used, in gigabytes to a tenth.
func swap() string {
	raw, err := unix.SysctlRaw("vm.swapusage")
	// total, available and used, each 64 bits: struct xsw_usage in
	// sys/sysctl.h
	if err != nil || len(raw) < 24 {
		return unknown
	}
	used := float64(binary.LittleEndian.Uint64(raw[16:24]))
	tenths := int(used/(1<<20)/102.4 + 0.5)
	return fmt.Sprintf("%d.%dg", tenths/10, tenths%10)
}

// disk is the space free on the volume that holds / and the home, in
// whole gigabytes.
func disk() string {
	var volume unix.Statfs_t
	if err := unix.Statfs("/", &volume); err != nil {
		return unknown
	}
	return fmt.Sprintf("%dg", volume.Bavail*uint64(volume.Bsize)>>30)
}

// trash is what emptying the Trash would give back, in gigabytes,
// rounded: a dash when that rounds to none, and unknown until it has
// been measured once or when it cannot be read.
func trash(now time.Time) string {
	bytes, known := trashed.size(now, trashDir())
	if !known {
		return unknown
	}
	return gigabytes(bytes)
}

// goCache returns the size of the go cache despite its confusing name
func goCache(now time.Time) string {
	dir := goCacheDir()
	if dir == "" {
		return unknown
	}
	bytes, known := goCached.size(now, dir)
	if !known {
		return unknown
	}
	return gigabytes(bytes)
}

// renderName uses the optional second line of the progress file.
// The same freshness and validity rules apply to both name and progress.
func renderName(path string, now time.Time) string {
	fields, name := renderFile(path)
	share, _, _ := renderFields(fields, now)
	if share == none || share == unknown {
		return share
	}
	if name == "" {
		return none
	}
	name = strings.Map(func(r rune) rune {
		if !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, name)
	if name == "" {
		return none
	}
	letters := []rune(name)
	if len(letters) <= 10 {
		return name
	}
	loop := []rune(name + "   ")
	start := int((now.Unix() / 2) % int64(len(loop)))
	return string(append(loop[start:], loop[:start]...))
}

// gigabytes is so many bytes as whole gigabytes, rounded, or a dash
// when that rounds to none.
func gigabytes(bytes int64) string {
	gigs := (bytes + 1<<29) >> 30
	if gigs == 0 {
		return none
	}
	return fmt.Sprintf("%dg", gigs)
}

// none is a reading there is nothing to take: no render is running.
const none = "-"

// render is a render's progress as three readings, each at most five
// cells: the share done as a percentage, the frames a minute, and the
// time left.
//
// With no file, or one its watcher has stopped keeping, all three are a
// dash. A file that cannot be understood is "?", never a number.
func render(path string, now time.Time) (share, rate, left string) {
	fields, _ := renderFile(path)
	return renderFields(fields, now)
}

// renderFields formats the first five whitespace-separated fields from the
// render progress file relative to now into display strings for completion
// percentage (capped at 100%), frames per minute, and remaining time.
//
// Expected field layout: <done> <total> <frames_per_minute> <eta_seconds>
// <epoch>. Non-negative integers are required except eta_seconds, which may be
// -1 when ETA is unavailable. Returns none for empty fields or stale updates
// (older than renderStale), and unknown for malformed input, fewer than five
// fields, or a zero total.
func renderFields(fields []string, now time.Time) (share, rate, left string) {
	if len(fields) == 0 {
		return none, none, none
	}
	if len(fields) < 5 {
		return unknown, unknown, unknown
	}
	var read [5]int64
	for i := range read {
		number, err := strconv.ParseInt(fields[i], 10, 64)
		// The time left alone may be -1: a watcher that cannot yet
		// say, as when nothing has finished and the rate is nought.
		if err != nil || number < -1 || (number == -1 && i != 3) {
			return unknown, unknown, unknown
		}
		read[i] = number
	}
	done, total, perMinute := read[0], read[1], read[2]
	seconds, taken := read[3], read[4]
	if total == 0 {
		return unknown, unknown, unknown
	}
	if now.Sub(time.Unix(taken, 0)) > renderStale {
		return none, none, none
	}
	percent := min(done*100/total, 100)
	left = unknown
	if seconds >= 0 {
		left = timeLeft(seconds)
	}
	rate = strconv.FormatInt(perMinute, 10)
	return fmt.Sprintf("%d%%", percent), rate, left
}

// timeLeft is a count of seconds in at most five cells: seconds under a
// minute, minutes under an hour, hours and minutes under a hundred
// hours, and days beyond.
func timeLeft(seconds int64) string {
	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dm", seconds/60)
	case seconds < 100*3600:
		return fmt.Sprintf("%dh%02d", seconds/3600, seconds%3600/60)
	}
	return fmt.Sprintf("%dd", seconds/86400)
}

// renderFile reads the progress numbers and optional name together. The
// file is opened inside its own folder so a link out is not followed.
func renderFile(path string) ([]string, string) {
	folder, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, ""
	}
	defer folder.Close()
	written, err := folder.ReadFile(filepath.Base(path))
	if err != nil {
		return nil, ""
	}
	first, rest, _ := strings.Cut(string(written), "\n")
	name, _, _ := strings.Cut(rest, "\n")
	return strings.Fields(first), strings.TrimSpace(name)
}

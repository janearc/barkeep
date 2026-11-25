package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// A sample is one reading and when it was taken.
type sample struct {
	at    time.Time
	value int
}

// A history is a reading's samples, oldest first, none older than it
// keeps. The strip takes its readings from one place, in turn, so a
// history is not shared and needs no lock.
type history struct {
	keeps   time.Duration
	samples []sample
}

// The readings worked from their own past: memory's free share, for its
// trend, and the GPU's busy share, for its mean.
var (
	memorySeen = history{keeps: trendBack}
	gpuSeen    = history{keeps: gpuOver}
)

// add puts a reading at the end and lets go of every one older than the
// history keeps.
func (seen *history) add(now time.Time, value int) {
	seen.samples = append(seen.samples, sample{now, value})
	first := 0
	for now.Sub(seen.samples[first].at) > seen.keeps {
		first++
	}
	seen.samples = seen.samples[first:]
}

// moved is how far the newest reading is from the oldest kept, and
// whether the oldest is at least so old: a history shorter than that
// does not go back far enough to say.
func (seen history) moved(now time.Time, least time.Duration) (int, bool) {
	if len(seen.samples) == 0 || now.Sub(seen.samples[0].at) < least {
		return 0, false
	}
	newest := seen.samples[len(seen.samples)-1]
	return newest.value - seen.samples[0].value, true
}

// mean is the readings kept, averaged and rounded to the nearest whole.
// It is asked only after a reading is added, so there is always one.
func (seen history) mean() int {
	sum := 0
	for _, each := range seen.samples {
		sum += each.value
	}
	count := len(seen.samples)
	return (sum + count/2) / count
}

// trashEvery is how often the Trash is measured. Measuring it reads the
// size of every file in it, which for a full Trash is not quick.
const trashEvery = 5 * time.Minute

// A measure is a folder's size, taken now and then by a walk that runs
// apart from whoever asks, so asking never waits on the walk.
type measure struct {
	sync.Mutex
	bytes   int64
	known   bool
	walking bool
	began   time.Time
}

// trashed is the Trash's size.
var trashed measure

// goCached is the Go build cache's last measured disk usage.
var goCached measure

// goCacheDir follows Go's GOCACHE override and its default cache folder.
func goCacheDir() string {
	if dir := os.Getenv("GOCACHE"); dir != "" {
		if filepath.IsAbs(dir) {
			return dir
		}
		return ""
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "go-build")
}

// size is the folder's size as last measured, and whether it has been
// measured and could be read. Asked when the last walk began longer ago
// than trashEvery, or never, it starts another.
func (kept *measure) size(now time.Time, dir string) (int64, bool) {
	kept.Lock()
	defer kept.Unlock()
	due := kept.began.IsZero() || now.Sub(kept.began) >= trashEvery
	if !kept.walking && due {
		kept.walking, kept.began = true, now
		go kept.walk(dir)
	}
	return kept.bytes, kept.known
}

// walk measures the folder and keeps what it found.
func (kept *measure) walk(dir string) {
	bytes, err := sizeOf(dir)
	kept.Lock()
	defer kept.Unlock()
	kept.bytes, kept.known, kept.walking = bytes, err == nil, false
}

// blockSize is the size of the blocks a file's count of them is in,
// whatever the volume's own: stat(2) counts in 512-byte blocks.
const blockSize = 512

// sizeOf is the space every file under a folder takes on its volume, in
// bytes: the blocks each holds, which is what removing it gives back,
// and not its length. A folder that cannot be listed is an error; a file
// inside it that cannot be read is passed over.
func sizeOf(dir string) (int64, error) {
	if _, err := os.ReadDir(dir); err != nil {
		return 0, err
	}
	var bytes int64
	err := filepath.WalkDir(dir, func(_ string, entry fs.DirEntry,
		err error) error {
		if err != nil {
			return nil
		}
		if info, err := entry.Info(); err == nil {
			if held, ok := info.Sys().(*syscall.Stat_t); ok {
				bytes += held.Blocks * blockSize
			}
		}
		return nil
	})
	return bytes, err
}

// trashDir is the Trash: the folder in the home that Finder moves what
// is deleted into.
func trashDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".Trash")
}

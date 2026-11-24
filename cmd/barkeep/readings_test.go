package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A history lets go of what is older than it keeps, and keeps the rest
// in the order it was given.
func TestAHistoryKeepsOnlyWhatIsNewEnough(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	seen := history{keeps: time.Minute}
	for second, value := range []int{10, 20, 30, 40} {
		seen.add(start.Add(time.Duration(second)*30*time.Second), value)
	}
	if len(seen.samples) != 3 || seen.samples[0].value != 20 ||
		seen.samples[2].value != 40 {
		t.Errorf("after ninety seconds a history of a minute holds %+v",
			seen.samples)
	}
}

// The trend is unknown until the oldest reading is old enough, then the
// change from it: nought steady, signed otherwise.
func TestTheTrendWaitsForAnOldEnoughReading(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	seen := history{keeps: trendBack}
	seen.add(start, 60)
	for _, each := range []struct {
		after time.Duration
		free  int
		want  string
	}{
		{trendMin - time.Second, 55, "(?)"},
		{trendMin, 55, "(-5)"},
		{trendMin + time.Second, 60, "(0)"},
		{trendMin + 2*time.Second, 63, "(+3)"},
	} {
		now := start.Add(each.after)
		seen.add(now, each.free)
		if got := trendOf(now, seen); got != each.want {
			t.Errorf("%v on, at %d free, the trend reads %s; want %s",
				each.after, each.free, got, each.want)
		}
	}
	if got := trendOf(start, history{}); got != "(?)" {
		t.Errorf("with no reading the trend reads %s", got)
	}
}

// and it is measured from the oldest reading still kept, so a reading
// that has aged out no longer counts.
func TestTheTrendForgetsWhatHasAgedOut(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	seen := history{keeps: trendBack}
	seen.add(start, 90)
	seen.add(start.Add(trendBack-trendMin), 50)
	now := start.Add(trendBack + time.Second)
	seen.add(now, 52)
	if got := trendOf(now, seen); got != "(+2)" {
		t.Errorf("the trend reads %s; want (+2), from 50 and not 90", got)
	}
}

// The mean is the kept readings averaged, rounded to the nearest whole.
func TestTheMeanIsRoundedToTheNearestWhole(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	for _, each := range []struct {
		values []int
		want   int
	}{{[]int{7}, 7}, {[]int{0, 1}, 1}, {[]int{1, 1, 2}, 1}, {[]int{10, 20, 90}, 40}} {
		seen := history{keeps: time.Hour}
		for _, value := range each.values {
			seen.add(start, value)
		}
		if got := seen.mean(); got != each.want {
			t.Errorf("the mean of %v is %d; want %d", each.values, got,
				each.want)
		}
	}
}

// Bytes read as whole gigabytes, rounded, and a dash when that is none.
func TestBytesReadAsRoundedGigabytes(t *testing.T) {
	for bytes, want := range map[int64]string{
		0: "-", 1<<29 - 1: "-", 1 << 29: "1g", 1 << 30: "1g",
		3<<30 + 1<<29: "4g", 45 << 30: "45g",
	} {
		if got := gigabytes(bytes); got != want {
			t.Errorf("%d bytes read %s; want %s", bytes, got, want)
		}
	}
}

// A folder's size is the blocks its files hold, through every folder
// under it; and a folder that is not there is an error, not nought.
func TestAFoldersSizeIsTheBlocksItsFilesHold(t *testing.T) {
	dir := t.TempDir()
	empty, err := sizeOf(dir)
	if err != nil {
		t.Fatal(err)
	}
	written := fill(t, dir)
	full, err := sizeOf(dir)
	if err != nil {
		t.Fatal(err)
	}
	if grew := full - empty; grew < written || grew > 2*written {
		t.Errorf("%d bytes written grew the folder by %d", written, grew)
	}
	if _, err := sizeOf(filepath.Join(dir, "not-here")); err == nil {
		t.Error("a folder that is not there has a size")
	}
}

// fill writes two files of a mebibyte each under a folder, one of them a
// folder down, and gives the bytes written.
func fill(t *testing.T, dir string) int64 {
	t.Helper()
	const each = 1 << 20
	below := filepath.Join(dir, "below")
	if err := os.Mkdir(below, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(dir, "one"),
		filepath.Join(below, "two")} {
		if err := os.WriteFile(path, make([]byte, each), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return 2 * each
}

// A measure is unknown until its first walk is done, known after, and
// does not walk again until trashEvery has gone by.
func TestAMeasureIsUnknownUntilItsFirstWalkIsDone(t *testing.T) {
	dir := t.TempDir()
	fill(t, dir)
	var kept measure
	start := time.Unix(1_000_000, 0)
	if _, known := kept.size(start, dir); known {
		t.Error("a measure never taken is known")
	}
	settled(t, &kept)
	first, known := kept.size(start.Add(time.Second), dir)
	if !known || first == 0 {
		t.Fatalf("after its walk the measure is %d, known %v", first, known)
	}
	os.Remove(filepath.Join(dir, "one"))
	if again, _ := kept.size(start.Add(trashEvery-time.Second), dir); again != first {
		t.Errorf("the measure changed to %d before it was due", again)
	}
	kept.size(start.Add(trashEvery), dir)
	settled(t, &kept)
	if later, _ := kept.size(start.Add(trashEvery+time.Second), dir); later >= first {
		t.Errorf("a due measure still reads %d after a file went; it was %d",
			later, first)
	}
}

// settled waits for a measure's walk to finish.
func settled(t *testing.T, kept *measure) {
	t.Helper()
	for range 2000 {
		kept.Lock()
		walking := kept.walking
		kept.Unlock()
		if !walking {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("the walk did not finish in two seconds")
}

// The GPU says how busy it is, as a percentage; and the strip's gpu
// reading is a whole number in that range.
func TestTheGPUSaysHowBusyItIs(t *testing.T) {
	busy, said := gpuBusy()
	if !said {
		t.Skip("no graphics accelerator publishes its use here")
	}
	if busy < 0 || busy > 100 {
		t.Errorf("the GPU says it is %d%% busy", busy)
	}
}

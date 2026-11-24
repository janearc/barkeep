package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// progress writes a render's progress file with these words and gives
// its path.
func progress(t *testing.T, words string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "render-progress.txt")
	if err := os.WriteFile(path, []byte(words+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A render a quarter done reads a quarter, its rate and its time left.
func TestARenderIsReadAsItsThreeRows(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	path := progress(t, fmt.Sprintf("93 372 12 1395 %d", now.Unix()-5))
	share, rate, left := render(path, now)
	if share != "25%" || rate != "12" || left != "23m" {
		t.Errorf("got %q %q %q, want 25%% 12 23m", share, rate, left)
	}
}

// A watcher that cannot yet say how long is left writes -1 there, and
// only there: the share and the rate are still read.
func TestATimeLeftNotYetKnownIsAQuestionMark(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	share, rate, left := render(progress(t, fmt.Sprintf("758 2090 0 -1 %d", now.Unix()-5)), now)
	if share != "36%" || rate != "0" || left != unknown {
		t.Errorf("got %q %q %q, want 36%% 0 ?", share, rate, left)
	}
	share, rate, left = render(progress(t, fmt.Sprintf("758 2090 -1 60 %d", now.Unix()-5)), now)
	if share != unknown || rate != unknown || left != unknown {
		t.Errorf("-1 as a rate: got %q %q %q, want a question mark each", share, rate, left)
	}
}

// The same render made wrong in one way is not read as a number: no
// file and a stopped watcher are a dash; a file that cannot be
// understood is a question mark.
func TestARenderMadeWrongIsNeverANumber(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	fresh := now.Unix() - 5
	for _, each := range []struct {
		name, words, want string
	}{
		{"a watcher that stopped", fmt.Sprintf("93 372 12 1395 %d", now.Unix()-121), none},
		{"an empty file", "", none},
		{"a field short", "93 372 12 1395", unknown},
		{"a word for a number", fmt.Sprintf("93 many 12 1395 %d", fresh), unknown},
		{"a negative count", fmt.Sprintf("-1 372 12 1395 %d", fresh), unknown},
		{"no frames in all", fmt.Sprintf("0 0 12 1395 %d", fresh), unknown},
	} {
		share, rate, left := render(progress(t, each.words), now)
		if share != each.want || rate != each.want || left != each.want {
			t.Errorf("%s: got %q %q %q, want %q each", each.name, share, rate, left, each.want)
		}
	}
	share, rate, left := render(filepath.Join(t.TempDir(), "missing"), now)
	if share != none || rate != none || left != none {
		t.Errorf("no file: got %q %q %q, want a dash each", share, rate, left)
	}
}

// A count past its total is whole, not more than whole, and a reading
// just inside the stale bound is still read.
func TestARenderAtItsEdges(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	share, _, _ := render(progress(t, fmt.Sprintf("400 372 12 0 %d", now.Unix()-120)), now)
	if share != "100%" {
		t.Errorf("past its total: got %q, want 100%%", share)
	}
}

// Every time left fits the strip's five cells and says the right unit.
func TestTimeLeftFitsFiveCells(t *testing.T) {
	for _, each := range []struct {
		seconds int64
		want    string
	}{
		{0, "0s"}, {59, "59s"}, {60, "1m"}, {3599, "59m"},
		{3600, "1h00"}, {3840, "1h04"}, {359_999, "99h59"},
		{360_000, "4d"}, {8_640_000, "100d"},
	} {
		got := timeLeft(each.seconds)
		if got != each.want {
			t.Errorf("%d seconds: got %q, want %q", each.seconds, got, each.want)
		}
		if len(got) > 5 {
			t.Errorf("%d seconds: %q is wider than five cells", each.seconds, got)
		}
	}
}

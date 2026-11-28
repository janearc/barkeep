package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	gameEvery = 4 * time.Second
	gameLimit = 2 * time.Second
	gamePage  = 8 * time.Second
	gameBytes = 256 * 1024
)

type gameBoard struct {
	queues []gameQueue
	jobs   []line
}

type gameQueue struct {
	name                        string
	open, running, waiting, max int
	runs, waits                 []line
}

var (
	queueHeader = regexp.MustCompile(
		`^([a-z][a-z0-9_-]*): (\d+) lanes?, (\d+) running, ` +
			`(\d+) waiting, at most (\d+)$`)
	jobDetail = regexp.MustCompile(`^(.+), pid ([1-9][0-9]*), .+$`)
)

// gameOutput bounds the memory we give a queue report, including commands.
type gameOutput struct{ buffer bytes.Buffer }

func (out *gameOutput) Write(data []byte) (int, error) {
	if len(data) > gameBytes-out.buffer.Len() {
		return 0, fmt.Errorf("game lanes report exceeds %d bytes", gameBytes)
	}
	return out.buffer.Write(data)
}

func readGame(ctx context.Context, game string) gameBoard {
	ctx, cancel := context.WithTimeout(ctx, gameLimit)
	defer cancel()
	out := &gameOutput{}
	cmd := exec.CommandContext(ctx, game, "lanes")
	cmd.Stdout = out
	cmd.WaitDelay = gameLimit
	if err := cmd.Run(); err != nil {
		return gameBoard{}
	}
	return parseGame(out.buffer.String())
}

// watchGame keeps the subprocess off the draw loop and samples one at a
// time. Every update is a whole board, with no stale counts on failure.
func watchGame(ctx context.Context, game string) <-chan gameBoard {
	updates := make(chan gameBoard, 1)
	go func() {
		tick := time.NewTicker(gameEvery)
		defer tick.Stop()
		for ctx.Err() == nil {
			board := readGame(ctx, game)
			select {
			case updates <- board:
			case <-ctx.Done():
				return
			}
			select {
			case <-tick.C:
			case <-ctx.Done():
				return
			}
		}
	}()
	return updates
}

// parseGame reads Game's human lane board. Check the known queues and
// every announced job count so partial output cannot pass for an idle queue.
func parseGame(output string) gameBoard {
	board := gameBoard{}
	seen := make(map[string]bool)
	var queue *gameQueue
	for row := range strings.SplitSeq(output, "\n") {
		if fields := queueHeader.FindStringSubmatch(row); fields != nil {
			name := fields[1]
			if seen[name] {
				return gameBoard{}
			}
			counts := [4]int{}
			for i, field := range fields[2:] {
				count, err := strconv.Atoi(field)
				if err != nil {
					return gameBoard{}
				}
				counts[i] = count
			}
			seen[name] = true
			board.queues = append(board.queues, gameQueue{
				name: name, open: counts[0], running: counts[1],
				waiting: counts[2], max: counts[3],
			})
			queue = &board.queues[len(board.queues)-1]
		} else if queue != nil {
			queue.addJob(row)
		}
	}
	for _, name := range []string{"mutants", "tests", "gpu"} {
		if !seen[name] {
			return gameBoard{}
		}
	}
	for _, queue := range board.queues {
		if len(queue.runs) != queue.running || len(queue.waits) != queue.waiting {
			return gameBoard{}
		}
		board.jobs = append(board.jobs, queue.runs...)
		board.jobs = append(board.jobs, queue.waits...)
	}
	return board
}

func (queue *gameQueue) addJob(row string) {
	mark := ">"
	jobs := &queue.runs
	rest, running := strings.CutPrefix(row, "  running: ")
	if !running {
		var waiting bool
		rest, waiting = strings.CutPrefix(row, "  waiting: ")
		if !waiting {
			return
		}
		mark, jobs = "+", &queue.waits
	}
	fields := jobDetail.FindStringSubmatch(rest)
	if fields == nil {
		return
	}
	title := gameJobName(fields[1])
	queueName := queue.name
	switch queueName {
	case "mutants", "tests", "gpu":
		queueName = queueName[:1]
	}
	*jobs = append(*jobs, line{
		name: queueName + mark + " " + title, reading: fields[2],
	})
}

// gameJobName keeps the identifying parts first, since a narrow strip
// has room for the shortest job name beside its PID.
func gameJobName(description string) string {
	command, directory, commanded := strings.Cut(description, " in /")
	name := filepath.Base(command)
	if commanded {
		words := strings.Fields(command)
		if len(words) > 0 {
			name = filepath.Base(words[0])
		}
		switch {
		case len(words) >= 3 && name == "azazel" && words[1] == "execute":
			last := words[len(words)-1]
			if _, err := strconv.ParseUint(last, 10, 64); err == nil {
				name += " #" + last
			}
		case len(words) >= 2 && !strings.HasPrefix(words[1], "-") &&
			!strings.Contains(words[1], "/"):
			name += " " + words[1]
		}
		name += " " + filepath.Base(directory)
	}
	return strings.Map(func(letter rune) rune {
		if letter < ' ' || letter > '~' {
			return '?'
		}
		return letter
	}, name)
}

// gameLines keeps the queue totals in view while excess jobs cycle
// through the remaining rows. The active page starts at the first job.
func (board gameBoard) gameLines(columns, rows int, elapsed time.Duration) []line {
	if rows <= 0 {
		return nil
	}
	if len(board.queues) == 0 {
		return []line{{name: "game", reading: unknown}}
	}
	legend := "run/q/open/max"
	if columns < len("game ")+len(legend) {
		legend = "r/q/o/m"
	}
	lines := []line{{name: "game", reading: legend}}
	for _, queue := range board.queues {
		counts := fmt.Sprintf("%d/%d/%d/%d", queue.running, queue.waiting,
			queue.open, queue.max)
		name := queue.name
		if name == "mutants" && columns < len(name)+gap+len(counts) {
			name = "mut"
		}
		lines = append(lines, line{name: name, reading: counts})
	}
	jobs := board.jobs
	room := rows - len(lines)
	switch {
	case room <= 0:
		return lines
	case len(jobs) <= room:
		return append(lines, jobs...)
	case room == 1:
		return append(lines, jobs[int(elapsed/gamePage)%len(jobs)])
	}
	room--
	pages := (len(jobs) + room - 1) / room
	page := int(elapsed/gamePage) % pages
	lines = append(lines, line{name: "jobs",
		reading: fmt.Sprintf("%d/%d", page+1, pages)})
	return append(lines, jobs[page*room:min((page+1)*room, len(jobs))]...)
}

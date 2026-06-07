package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const barWidth = 35

// stderrIsTTY is true when stderr is an interactive terminal.
var stderrIsTTY = func() bool {
	fi, err := os.Stderr.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}()

type progressBar struct {
	label   string
	total   int64
	done    int64
	lastPct int // last rendered percentage * 2 (0.5% granularity)
}

func newProgressBar(label string, totalBytes int64) *progressBar {
	pb := &progressBar{label: label, total: totalBytes, lastPct: -1}
	if stderrIsTTY {
		pb.render()
	} else {
		fmt.Fprintf(os.Stderr, "%s\n", label)
	}
	return pb
}

func (pb *progressBar) render() {
	pct := 0.0
	if pb.total > 0 {
		pct = float64(pb.done) / float64(pb.total)
		if pct > 1 {
			pct = 1
		}
	}
	filled := int(pct * barWidth)
	var b strings.Builder
	b.WriteString(strings.Repeat("=", filled))
	if filled < barWidth {
		b.WriteByte('>')
		b.WriteString(strings.Repeat(" ", barWidth-filled-1))
	}
	fmt.Fprintf(os.Stderr, "\r%-20s [%s] %3.0f%%", pb.label, b.String(), pct*100)
}

func (pb *progressBar) update(n int64) {
	pb.done += n
	if !stderrIsTTY {
		return
	}
	// Re-render at 0.5% granularity to avoid excessive writes.
	newPct := int(float64(pb.done) / float64(pb.total) * 200)
	if newPct != pb.lastPct {
		pb.lastPct = newPct
		pb.render()
	}
}

func (pb *progressBar) finish() {
	if stderrIsTTY {
		pb.done = pb.total
		pb.render()
		fmt.Fprintln(os.Stderr)
	} else {
		fmt.Fprintf(os.Stderr, "  done\n")
	}
}

// progressReader wraps an io.Reader and ticks a progress bar as bytes flow through.
type progressReader struct {
	r   io.Reader
	bar *progressBar
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.r.Read(p)
	if n > 0 {
		pr.bar.update(int64(n))
	}
	return
}

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

package gedcom

import (
	"fmt"
	"io"
	"strings"
)

const maxLineLen = 255

// WriteRecord writes a Node tree to w in GEDCOM format.
func WriteRecord(w io.Writer, n *Node) error {
	return writeNode(w, n)
}

func writeNode(w io.Writer, n *Node) error {
	line := buildLine(n)
	// GEDCOM 5.5.1 long-line splitting (CONC continuation)
	lines := splitLongLine(n.Level, n.Tag, line)
	for _, l := range lines {
		if _, err := fmt.Fprintln(w, l); err != nil {
			return err
		}
	}
	for _, child := range n.Children {
		if err := writeNode(w, child); err != nil {
			return err
		}
	}
	return nil
}

func buildLine(n *Node) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d", n.Level)
	if n.XRef != "" {
		fmt.Fprintf(&sb, " %s", n.XRef)
	}
	fmt.Fprintf(&sb, " %s", n.Tag)
	if n.Value != "" {
		fmt.Fprintf(&sb, " %s", n.Value)
	}
	return sb.String()
}

// splitLongLine splits lines exceeding maxLineLen using CONC continuations.
// Splitting is done at rune (Unicode character) boundaries to avoid corrupting
// multi-byte UTF-8 sequences.
func splitLongLine(level int, tag, line string) []string {
	runes := []rune(line)
	if len(runes) <= maxLineLen {
		return []string{line}
	}
	var result []string
	result = append(result, string(runes[:maxLineLen]))
	remainder := runes[maxLineLen:]
	prefix := []rune(fmt.Sprintf("%d CONC ", level+1))
	for len(remainder) > 0 {
		avail := maxLineLen - len(prefix)
		if avail <= 0 {
			avail = 1
		}
		if len(remainder) <= avail {
			result = append(result, string(prefix)+string(remainder))
			break
		}
		result = append(result, string(prefix)+string(remainder[:avail]))
		remainder = remainder[avail:]
	}
	return result
}

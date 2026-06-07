package gedcom

import (
	"strings"
)

// Node represents one GEDCOM line and its children.
type Node struct {
	Level    int
	XRef     string // e.g. "@I123@", only on level-0 records
	Tag      string
	Value    string
	Children []*Node
}

// parseLine parses a single GEDCOM line of the form:
//
//	LEVEL [XREF] TAG [VALUE]
func parseLine(raw string) *Node {
	raw = strings.TrimRight(raw, "\r\n")
	if raw == "" {
		return nil
	}
	parts := strings.SplitN(raw, " ", 3)
	if len(parts) < 2 {
		return nil
	}
	n := &Node{}
	// level
	for _, ch := range parts[0] {
		if ch < '0' || ch > '9' {
			return nil
		}
		n.Level = n.Level*10 + int(ch-'0')
	}
	rest := parts[1:]
	// optional xref (@...@)
	if strings.HasPrefix(rest[0], "@") && strings.HasSuffix(rest[0], "@") {
		n.XRef = rest[0]
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return nil
	}
	n.Tag = rest[0]
	if len(rest) > 1 {
		n.Value = rest[1]
	}
	return n
}

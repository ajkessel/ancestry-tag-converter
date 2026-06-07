package gedcom

import (
	"bufio"
	"io"
	"strings"
)

// Parser streams GEDCOM records one level-0 block at a time.
type Parser struct {
	scanner *bufio.Scanner
	pending *Node // buffered level-0 line for next record
	done    bool
}

func NewParser(r io.Reader) *Parser {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB line buffer
	return &Parser{scanner: sc}
}

// Next returns the next top-level record, or nil when the file is exhausted.
func (p *Parser) Next() *Node {
	if p.done {
		return nil
	}

	var root *Node
	var stack []*Node // stack[0] = root, stack[n] = current deepest node

	// If we have a buffered level-0 line from the previous call, start with it.
	if p.pending != nil {
		root = p.pending
		p.pending = nil
		stack = []*Node{root}
	}

	for p.scanner.Scan() {
		line := p.scanner.Text()
		// Skip BOM on first line
		line = strings.TrimPrefix(line, "\xef\xbb\xbf")

		n := parseLine(line)
		if n == nil {
			continue
		}

		if n.Level == 0 {
			if root == nil {
				// First record
				root = n
				stack = []*Node{root}
				continue
			}
			// New record starts — buffer it and return the current one
			p.pending = n
			return root
		}

		// Find parent: walk stack back to the node whose level == n.Level-1
		for len(stack) > 1 && stack[len(stack)-1].Level >= n.Level {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		parent.Children = append(parent.Children, n)
		stack = append(stack, n)
	}

	p.done = true
	return root // may be nil if file was empty
}

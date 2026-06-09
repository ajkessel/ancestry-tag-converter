package help

import _ "embed"

// Markdown contains the help shown by the GUI.
//
//go:embed HELP.md
var Markdown string

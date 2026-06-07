package converter

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ajkessel/ancestry-tag-converter/gedcom"
)

// singletonTags are event tags that should appear at most once per individual.
// If the FTM base already has one, we skip adding the Ancestry version.
var singletonTags = map[string]bool{
	"BIRT": true,
	"DEAT": true,
	"CHR":  true,
	"BAPM": true,
	"BURI": true,
	"FCOM": true,
	"CONF": true,
}

// skipMergeTags are tags never copied from Ancestry during merge — they are
// structural (family links, IDs) or always present in the FTM base.
var skipMergeTags = map[string]bool{
	"NAME": true,
	"SEX":  true,
	"FAMC": true,
	"FAMS": true,
	"REFN": true,
	"RIN":  true,
	"CHAN": true,
	"OBJE": true, // FTM manages its own media links
	"SOUR": true, // Ancestry SOUR XRefs don't exist in the FTM file; skip to avoid dangling references
}

// IndexedGEDCOM holds all parsed records from a GEDCOM file, indexed for lookup.
type IndexedGEDCOM struct {
	Records   []*gedcom.Node          // all records in original order (includes TRLR)
	ByXRef    map[string]*gedcom.Node // xref → record
	IndiByKey map[string]*gedcom.Node // match key → INDI record
	Warnings  []string                // non-fatal issues found during indexing
}

// LoadAndIndex reads an entire GEDCOM file into memory and builds lookup indexes.
func LoadAndIndex(path string) (*IndexedGEDCOM, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadAndIndexFromReader(f)
}

// LoadAndIndexFromReader is the reader-based variant of LoadAndIndex.
func LoadAndIndexFromReader(r io.Reader) (*IndexedGEDCOM, error) {
	g := &IndexedGEDCOM{
		ByXRef:    make(map[string]*gedcom.Node),
		IndiByKey: make(map[string]*gedcom.Node),
	}

	p := gedcom.NewParser(r)
	for {
		rec := p.Next()
		if rec == nil {
			break
		}
		g.Records = append(g.Records, rec)
		if rec.XRef != "" {
			g.ByXRef[rec.XRef] = rec
		}
		if rec.Tag == "INDI" {
			key := IndividualKey(rec)
			if key != "" {
				if existing, dup := g.IndiByKey[key]; dup {
					g.Warnings = append(g.Warnings, fmt.Sprintf(
						"duplicate individuals in base file share key %q: %s and %s",
						key, existing.XRef, rec.XRef,
					))
				}
				g.IndiByKey[key] = rec
			}
		}
	}
	return g, nil
}

// FuzzyMatchINDI finds an INDI record whose compact name contains (or is
// contained by) the compact name of n, falling back when exact key lookup
// fails. This handles FTM silently stripping backslashes — "Pinkas\Pinkhas"
// becomes "PinkasPinkhas" — which may also drop trailing name segments
// entirely (e.g. "\Lewin" → gone). Both names must share a compact-name
// prefix of at least 8 characters, and if both have birth years they must
// agree.
//
// Returns the matched record (or nil) and a boolean that is true when more
// than one candidate matched — callers should warn the user in that case.
func (g *IndexedGEDCOM) FuzzyMatchINDI(n *gedcom.Node) (*gedcom.Node, bool) {
	qCompact := compactName(n)
	if len(qCompact) < 8 {
		return nil, false
	}
	qYear := birthYear(n)
	var matches []*gedcom.Node
	for _, rec := range g.IndiByKey {
		rCompact := compactName(rec)
		if len(rCompact) < 8 {
			continue
		}
		if !strings.HasPrefix(qCompact, rCompact) && !strings.HasPrefix(rCompact, qCompact) {
			continue
		}
		rYear := birthYear(rec)
		if qYear != "" && rYear != "" && qYear != rYear {
			continue
		}
		matches = append(matches, rec)
	}
	switch len(matches) {
	case 0:
		return nil, false
	case 1:
		return matches[0], false
	default:
		return matches[0], true
	}
}

// compactName strips every non-letter character from the normalized name,
// producing a run of lowercase letters with no spaces or punctuation.
func compactName(n *gedcom.Node) string {
	name := normalizedName(n)
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// IndividualKey returns a canonical match key for an INDI node:
// normalized_name|birth_year, or just normalized_name if no birth year found.
func IndividualKey(n *gedcom.Node) string {
	name := normalizedName(n)
	if name == "" {
		return ""
	}
	year := birthYear(n)
	if year == "" {
		return name
	}
	return name + "|" + year
}

func normalizedName(n *gedcom.Node) string {
	for _, c := range n.Children {
		if c.Tag == "NAME" {
			s := c.Value
			s = strings.ReplaceAll(s, "/", " ")
			s = strings.ReplaceAll(s, "\\", " ")
			s = strings.Join(strings.Fields(s), " ")
			return strings.ToLower(s)
		}
	}
	return ""
}

func birthYear(n *gedcom.Node) string {
	for _, c := range n.Children {
		if c.Tag == "BIRT" {
			for _, gc := range c.Children {
				if gc.Tag == "DATE" {
					return extractYear(gc.Value)
				}
			}
		}
	}
	return ""
}

// extractYear pulls a 4-digit year from a GEDCOM date string (e.g. "13 Jun 1976" → "1976").
func extractYear(s string) string {
	for _, part := range strings.Fields(normalizeDate(s)) {
		if len(part) == 4 && part >= "1000" && part <= "2100" {
			allDigits := true
			for _, r := range part {
				if r < '0' || r > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return part
			}
		}
	}
	return ""
}

var gedcomMonths = [...]string{
	"", "jan", "feb", "mar", "apr", "may", "jun",
	"jul", "aug", "sep", "oct", "nov", "dec",
}

// monthAbbrev maps full month names and standard abbreviations to 3-letter
// lowercase abbreviations used in canonical GEDCOM dates.
var monthAbbrev = map[string]string{
	"january": "jan", "jan": "jan",
	"february": "feb", "feb": "feb",
	"march": "mar", "mar": "mar",
	"april": "apr", "apr": "apr",
	"may": "may",
	"june": "jun", "jun": "jun",
	"july": "jul", "jul": "jul",
	"august": "aug", "aug": "aug",
	"september": "sep", "sept": "sep", "sep": "sep",
	"october": "oct", "oct": "oct",
	"november": "nov", "nov": "nov",
	"december": "dec", "dec": "dec",
}

// normalizeDate converts various date representations to lowercase standard GEDCOM form
// so that equivalent dates compare equal regardless of source format.
// "10/2/1905", "2 OCT 1905", and "March 5,1882" all normalize to canonical form.
func normalizeDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Handle M/D/YYYY slash format produced by Ancestry exports.
	if parts := strings.Split(s, "/"); len(parts) == 3 {
		m, err1 := strconv.Atoi(parts[0])
		d, err2 := strconv.Atoi(parts[1])
		y, err3 := strconv.Atoi(parts[2])
		if err1 == nil && err2 == nil && err3 == nil && m >= 1 && m <= 12 {
			if y < 100 {
				y += 1900
			}
			return fmt.Sprintf("%d %s %d", d, gedcomMonths[m], y)
		}
	}
	// Parse token-by-token to handle "March 5,1882", "5 MAR 1882", etc.
	// Commas are treated as whitespace (e.g. "March 5,1882" → "March 5 1882").
	tokens := strings.Fields(strings.ToLower(strings.ReplaceAll(s, ",", " ")))
	var year, day int
	var month string
	for _, tok := range tokens {
		if abbr, ok := monthAbbrev[tok]; ok {
			month = abbr
		} else if n, err := strconv.Atoi(tok); err == nil {
			switch {
			case n >= 1000 && n <= 2100:
				year = n
			case n >= 1 && n <= 31 && day == 0:
				day = n
			}
		}
	}
	if month != "" && year != 0 {
		if day != 0 {
			return fmt.Sprintf("%d %s %d", day, month, year)
		}
		return fmt.Sprintf("%s %d", month, year)
	}
	// Fallback: normalize whitespace and case only.
	return strings.ToLower(strings.Join(tokens, " "))
}

// MergeINDI adds non-duplicate events from src (converted Ancestry) into dst (FTM base).
// Returns the number of events added.
func MergeINDI(dst, src *gedcom.Node, stats *Stats) int {
	added := 0
	existing := buildExistingSet(dst)

	for _, child := range src.Children {
		if skipMergeTags[child.Tag] {
			continue
		}
		// Singleton: skip if dst already has this event type
		if singletonTags[child.Tag] {
			if _, has := existing["singleton:"+child.Tag]; has {
				continue
			}
		}
		sig := eventSignature(child)
		if _, dup := existing[sig]; dup {
			continue
		}
		dst.Children = append(dst.Children, child)
		existing[sig] = struct{}{}
		stats.Converted["merge:"+child.Tag]++
		added++
	}
	return added
}

// buildExistingSet returns the set of deduplication keys for all events in an INDI.
func buildExistingSet(n *gedcom.Node) map[string]struct{} {
	set := make(map[string]struct{})
	for _, c := range n.Children {
		if singletonTags[c.Tag] {
			set["singleton:"+c.Tag] = struct{}{}
		}
		set[eventSignature(c)] = struct{}{}
	}
	return set
}

// eventSignature returns a canonical deduplication string for an event node.
// Two events with the same signature are considered duplicates.
func eventSignature(n *gedcom.Node) string {
	date := normalizeDate(childValue(n, "DATE"))
	plac := strings.ToLower(strings.TrimSpace(childValue(n, "PLAC")))
	typ := strings.ToLower(strings.TrimSpace(childValue(n, "TYPE")))
	val := strings.ToLower(strings.TrimSpace(n.Value))

	var b strings.Builder
	b.WriteString(strings.ToUpper(n.Tag))
	if val != "" {
		b.WriteString("|val:")
		b.WriteString(val)
	}
	if typ != "" {
		b.WriteString("|type:")
		b.WriteString(typ)
	}
	if date != "" {
		b.WriteString("|date:")
		b.WriteString(date)
	}
	if plac != "" {
		b.WriteString("|plac:")
		b.WriteString(plac)
	}
	return b.String()
}

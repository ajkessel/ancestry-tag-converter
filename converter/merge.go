package converter

import (
	"io"
	"os"
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
				g.IndiByKey[key] = rec
			}
		}
	}
	return g, nil
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
	for _, part := range strings.Fields(s) {
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
	date := strings.ToLower(strings.Join(strings.Fields(childValue(n, "DATE")), " "))
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

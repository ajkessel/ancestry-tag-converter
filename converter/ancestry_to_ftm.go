package converter

import (
	"strings"
	"unicode"

	"github.com/ajkessel/ancestry-tag-converter/gedcom"
)

// Stats tracks what the converter did.
type Stats struct {
	Records  map[string]int
	Dropped  map[string]int
	Converted map[string]int
}

func NewStats() *Stats {
	return &Stats{
		Records:  make(map[string]int),
		Dropped:  make(map[string]int),
		Converted: make(map[string]int),
	}
}

// dropTags lists tags that should be removed entirely (along with their children).
// These are all Ancestry-internal with no meaningful FTM equivalent.
var dropTags = map[string]bool{
	"_APID":  true,
	"_HPID":  true,
	"_WPID":  true,
	"_MTTAG": true,
	"_OID":   true,
	"_META":  true,
	"_CREA":  true,
	"_USER":  true,
	"_ENCR":  true,
	"_CLON":  true,
	"_ORIG":  true,
	"_MSER":  true,
	"_ATL":   true,
	"_PRIM":  true,
	"_STYPE": true,
	"_WDTH":  true,
	"_HGHT":  true,
	"_SIZE":  true,
	"_MTYPE": true,
	"_TREE":  true,
	"_ENV":   true,
}

// Options controls optional conversion behavior.
type Options struct {
	NoFRel  bool // don't add _FREL/_MREL to CHIL
	NoMedia bool // drop all OBJE records
}

// Convert transforms a single Ancestry GEDCOM record to FTM-compatible form.
// Returns nil if the record should be dropped entirely.
func Convert(n *gedcom.Node, stats *Stats, opts Options) *gedcom.Node {
	if n == nil {
		return nil
	}
	stats.Records[n.Tag]++

	switch n.Tag {
	case "HEAD":
		return convertHEAD(n, stats)
	case "INDI":
		return convertINDI(n, stats)
	case "FAM":
		return convertFAM(n, stats, opts)
	case "OBJE":
		if opts.NoMedia {
			stats.Dropped["OBJE"]++
			return nil
		}
		return convertOBJE(n, stats)
	case "SOUR":
		return convertSOUR(n, stats)
	case "_MTTAG", "_MTCAT":
		// Ancestry-specific DNA matching records — no FTM equivalent
		stats.Dropped[n.Tag]++
		return nil
	case "TRLR":
		// Dropped here; main.go writes the trailer after all records
		return nil
	default:
		return filterChildren(n, stats)
	}
}

// convertHEAD rebuilds the HEAD record, stripping Ancestry-specific content.
func convertHEAD(n *gedcom.Node, stats *Stats) *gedcom.Node {
	out := &gedcom.Node{Level: 0, Tag: "HEAD"}
	for _, child := range n.Children {
		switch child.Tag {
		case "SOUR":
			// Replace Ancestry SOUR with a minimal source block
			sour := &gedcom.Node{Level: 1, Tag: "SOUR", Value: "ancestry-to-ftm"}
			sour.Children = []*gedcom.Node{
				{Level: 2, Tag: "NAME", Value: "ancestry-to-ftm GEDCOM converter"},
			}
			out.Children = append(out.Children, sour)
		case "SUBM", "GEDC", "CHAR", "DATE", "FILE", "DEST", "NOTE":
			out.Children = append(out.Children, child)
		// Drop SOUR-adjacent Ancestry fields that leak to HEAD level
		default:
			// keep anything else (future-proofing)
			out.Children = append(out.Children, child)
		}
	}
	return out
}

// convertINDI processes an individual record.
func convertINDI(n *gedcom.Node, stats *Stats) *gedcom.Node {
	out := shallowCopy(n)
	for _, child := range n.Children {
		if dropTags[child.Tag] {
			stats.Dropped[child.Tag]++
			continue
		}
		switch child.Tag {
		case "GRAD":
			out.Children = append(out.Children, convertGRAD(child, stats))
		case "NAME", "BIRT", "DEAT", "BURI", "BAPM", "CONF", "BARM", "RESI",
			"OCCU", "EMIG", "IMMI", "NATU", "EVEN", "MARR", "DIV", "ENGA",
			"EDUC", "RELI", "TITL", "CENS", "WILL", "PROB", "ADOP", "CHR",
			"CHRA", "FCOM", "ORDN", "ORDI", "MILI", "_MILT", "_NAMS", "_URL",
			"SEX", "FAMC", "FAMS", "SOUR", "NOTE", "OBJE", "CHAN", "REFN",
			"ASSO", "ALIA", "ANCI", "DESI", "RFN", "AFN", "RIN",
			"_DCAUSE", "_FUN", "_LINK", "_PHOTO":
			out.Children = append(out.Children, filterSOURCitations(child, stats))
		default:
			out.Children = append(out.Children, filterChildren(child, stats))
		}
	}
	return out
}

// convertFAM processes a family record.
func convertFAM(n *gedcom.Node, stats *Stats, opts Options) *gedcom.Node {
	out := shallowCopy(n)
	for _, child := range n.Children {
		if dropTags[child.Tag] {
			stats.Dropped[child.Tag]++
			continue
		}
		if child.Tag == "CHIL" {
			out.Children = append(out.Children, child)
			if !opts.NoFRel {
				frel := &gedcom.Node{Level: child.Level + 1, Tag: "_FREL", Value: "Natural"}
				mrel := &gedcom.Node{Level: child.Level + 1, Tag: "_MREL", Value: "Natural"}
				out.Children = append(out.Children, frel, mrel)
				stats.Converted["CHIL→_FREL/_MREL"]++
			}
			continue
		}
		out.Children = append(out.Children, filterSOURCitations(child, stats))
	}
	return out
}

// convertOBJE strips Ancestry-internal OBJE metadata, keeping the skeleton.
func convertOBJE(n *gedcom.Node, stats *Stats) *gedcom.Node {
	out := shallowCopy(n)
	for _, child := range n.Children {
		if dropTags[child.Tag] {
			stats.Dropped[child.Tag]++
			continue
		}
		switch child.Tag {
		case "FILE":
			out.Children = append(out.Children, convertFILE(child, stats))
		case "TITL", "NOTE", "_DATE", "_TEXT", "_DSCR", "PLAC":
			out.Children = append(out.Children, child)
		case "DATE":
			// FTM uses _DATE for media dates; convert from standard DATE
			converted := shallowCopy(child)
			converted.Tag = "_DATE"
			out.Children = append(out.Children, converted)
			stats.Converted["OBJE DATE→_DATE"]++
		default:
			// drop unknown OBJE children
			stats.Dropped[child.Tag]++
		}
	}
	return out
}

// convertFILE strips Ancestry image-metadata tags from FILE children.
func convertFILE(n *gedcom.Node, stats *Stats) *gedcom.Node {
	out := shallowCopy(n)
	for _, child := range n.Children {
		if dropTags[child.Tag] {
			stats.Dropped[child.Tag]++
			continue
		}
		if child.Tag == "FORM" {
			// Keep FORM but strip its custom children
			formOut := shallowCopy(child)
			for _, fc := range child.Children {
				if fc.Tag == "TYPE" {
					formOut.Children = append(formOut.Children, fc)
				} else {
					stats.Dropped[fc.Tag]++
				}
			}
			out.Children = append(out.Children, formOut)
		} else {
			out.Children = append(out.Children, child)
		}
	}
	return out
}

// convertSOUR processes a top-level SOUR record.
func convertSOUR(n *gedcom.Node, stats *Stats) *gedcom.Node {
	out := shallowCopy(n)
	for _, child := range n.Children {
		if dropTags[child.Tag] {
			stats.Dropped[child.Tag]++
			continue
		}
		out.Children = append(out.Children, filterChildren(child, stats))
	}
	return out
}

// convertGRAD moves a NOTE child that looks like a school name onto the GRAD value.
//
//	Ancestry: 1 GRAD  /  2 NOTE School Name
//	FTM:      1 GRAD School Name
func convertGRAD(n *gedcom.Node, stats *Stats) *gedcom.Node {
	out := shallowCopy(n)
	for _, child := range n.Children {
		if child.Tag == "NOTE" && out.Value == "" && looksLikeSchoolName(child.Value) {
			out.Value = child.Value
			stats.Converted["GRAD/NOTE→value"]++
			continue
		}
		out.Children = append(out.Children, filterChildren(child, stats))
	}
	return out
}

// filterSOURCitations recursively filters an event node that may contain
// inline SOUR citations with DATA/WWW sub-trees and _APID tags.
func filterSOURCitations(n *gedcom.Node, stats *Stats) *gedcom.Node {
	if n.Tag == "SOUR" {
		return convertInlineSOUR(n, stats)
	}
	out := shallowCopy(n)
	for _, child := range n.Children {
		if dropTags[child.Tag] {
			stats.Dropped[child.Tag]++
			continue
		}
		out.Children = append(out.Children, filterSOURCitations(child, stats))
	}
	return out
}

// convertInlineSOUR converts an inline SOUR citation:
// - drops _APID, _HPID, _WPID
// - converts DATA/WWW → _LINK + NOTE
func convertInlineSOUR(n *gedcom.Node, stats *Stats) *gedcom.Node {
	out := shallowCopy(n)
	for _, child := range n.Children {
		if dropTags[child.Tag] {
			stats.Dropped[child.Tag]++
			continue
		}
		if child.Tag == "DATA" {
			// Look for WWW grandchildren
			url := extractWWW(child)
			if url != "" {
				linkLevel := child.Level
				out.Children = append(out.Children,
					&gedcom.Node{Level: linkLevel, Tag: "_LINK", Value: url},
					&gedcom.Node{Level: linkLevel, Tag: "NOTE", Value: url},
				)
				stats.Converted["DATA/WWW→_LINK"]++
				// Also preserve non-WWW children of DATA
				for _, dc := range child.Children {
					if dc.Tag != "WWW" {
						out.Children = append(out.Children, dc)
					}
				}
				continue
			}
			// No WWW — keep DATA as-is
			out.Children = append(out.Children, child)
			continue
		}
		out.Children = append(out.Children, filterChildren(child, stats))
	}
	return out
}

// extractWWW returns the value of the first WWW child of a DATA node, or "".
func extractWWW(data *gedcom.Node) string {
	for _, child := range data.Children {
		if child.Tag == "WWW" {
			return child.Value
		}
	}
	return ""
}

// filterChildren recursively removes dropTags from a node's subtree.
func filterChildren(n *gedcom.Node, stats *Stats) *gedcom.Node {
	out := shallowCopy(n)
	for _, child := range n.Children {
		if dropTags[child.Tag] {
			stats.Dropped[child.Tag]++
			continue
		}
		out.Children = append(out.Children, filterChildren(child, stats))
	}
	return out
}

func shallowCopy(n *gedcom.Node) *gedcom.Node {
	return &gedcom.Node{
		Level: n.Level,
		XRef:  n.XRef,
		Tag:   n.Tag,
		Value: n.Value,
	}
}

// looksLikeSchoolName returns true if s is a short, mostly-proper-noun phrase
// (not a multi-sentence note). A school name has no periods mid-sentence and
// is under 120 characters.
func looksLikeSchoolName(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 || len(s) > 120 {
		return false
	}
	// Reject if it looks like a sentence (contains period not at end)
	trimmed := strings.TrimRight(s, ". ")
	if strings.Contains(trimmed, ". ") {
		return false
	}
	// Must start with an uppercase letter (proper noun)
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}

package converter

import (
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/ajkessel/ancestry-tag-converter/gedcom"
)

// MTTagInfo holds the resolved name and category for one _MTTAG definition.
type MTTagInfo struct {
	Name    string
	CatXRef string
	Note    string
	Record  *gedcom.Node
}

// MTCatInfo holds the resolved name and original record for one _MTCAT definition.
type MTCatInfo struct {
	Name   string
	Record *gedcom.Node
}

// ScanMTTags does a fast first-pass over the GEDCOM file to collect all
// _MTTAG and _MTCAT records. Returns maps of XRef → info / name.
func ScanMTTags(path string) (mttagMap map[string]MTTagInfo, mtcatMap map[string]MTCatInfo, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	return scanMTTagsFromReader(f)
}

// ScanMTTagsFromReader is the reader-based variant of ScanMTTags.
func ScanMTTagsFromReader(r io.Reader) (map[string]MTTagInfo, map[string]MTCatInfo, error) {
	return scanMTTagsFromReader(r)
}

func scanMTTagsFromReader(r io.Reader) (map[string]MTTagInfo, map[string]MTCatInfo, error) {
	mttagMap := make(map[string]MTTagInfo)
	mtcatMap := make(map[string]MTCatInfo)

	p := gedcom.NewParser(r)
	for {
		rec := p.Next()
		if rec == nil {
			break
		}
		switch rec.Tag {
		case "_MTTAG":
			info := MTTagInfo{CatXRef: childValue(rec, "_MTCAT"), Record: cloneNode(rec)}
			info.Name = childValue(rec, "NAME")
			info.Note = childValue(rec, "NOTE")
			mttagMap[rec.XRef] = info
		case "_MTCAT":
			mtcatMap[rec.XRef] = MTCatInfo{Name: childValue(rec, "NAME"), Record: cloneNode(rec)}
		}
	}
	return mttagMap, mtcatMap, nil
}

// childValue returns the Value of the first child with the given tag, or "".
func childValue(n *gedcom.Node, tag string) string {
	for _, c := range n.Children {
		if c.Tag == tag {
			return c.Value
		}
	}
	return ""
}

// Stats tracks what the converter did.
type Stats struct {
	Records   map[string]int
	Dropped   map[string]int
	Converted map[string]int
}

func NewStats() *Stats {
	return &Stats{
		Records:   make(map[string]int),
		Dropped:   make(map[string]int),
		Converted: make(map[string]int),
	}
}

// dropTags lists tags that should be removed entirely (along with their children).
// These are all Ancestry-internal with no meaningful FTM equivalent.
var dropTags = map[string]bool{
	"_APID": true,
	"_HPID": true,
	"_WPID": true,
	// _MTTAG is NOT in dropTags — inline refs on INDI are converted to FACT
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
	"_WLNK":  true,
}

type OriginalDataMode string

const (
	OriginalDataKeep    OriginalDataMode = "keep"
	OriginalDataDiscard OriginalDataMode = "discard"
)

type CustomTagRecord string

const (
	CustomTagFact  CustomTagRecord = "fact"
	CustomTagEvent CustomTagRecord = "event"
)

// Options controls optional conversion behavior. Empty mode values use the
// user-facing defaults: keep original data and emit FACT records.
type Options struct {
	NoFRel          bool // don't add _FREL/_MREL to CHIL
	NoMedia         bool // drop all OBJE records
	OriginalData    OriginalDataMode
	CustomTagRecord CustomTagRecord
	MTTagMap        map[string]MTTagInfo // XRef → tag info (from first-pass scan)
	MTCatMap        map[string]MTCatInfo // XRef → category info
}

func (o Options) keepOriginalData() bool {
	return o.OriginalData == "" || o.OriginalData == OriginalDataKeep
}

func (o Options) customTagGEDCOMTag() string {
	if o.CustomTagRecord == CustomTagEvent {
		return "EVEN"
	}
	return "FACT"
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
		if opts.keepOriginalData() {
			return cloneNode(n)
		}
		return convertHEAD(n, stats)
	case "INDI":
		return convertINDI(n, stats, opts)
	case "FAM":
		return convertFAM(n, stats, opts)
	case "OBJE":
		if opts.NoMedia {
			stats.Dropped["OBJE"]++
			return nil
		}
		if opts.keepOriginalData() {
			return preserveOBJE(n, stats)
		}
		return convertOBJE(n, stats)
	case "SOUR":
		if opts.keepOriginalData() {
			return preserveAndConvertSOURCitations(n, stats)
		}
		return convertSOUR(n, stats)
	case "_MTTAG", "_MTCAT":
		if opts.keepOriginalData() {
			return cloneNode(n)
		}
		stats.Dropped[n.Tag]++
		return nil
	case "TRLR":
		// Dropped here; main.go writes the trailer after all records
		return nil
	default:
		if opts.keepOriginalData() {
			return cloneNode(n)
		}
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
			sour := &gedcom.Node{Level: 1, Tag: "SOUR", Value: "ancestry-tag-converter"}
			sour.Children = []*gedcom.Node{
				{Level: 2, Tag: "NAME", Value: "ancestry-tag-converter GEDCOM converter"},
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
func convertINDI(n *gedcom.Node, stats *Stats, opts Options) *gedcom.Node {
	out := shallowCopy(n)
	for _, child := range n.Children {
		if opts.NoMedia && child.Tag == "OBJE" {
			stats.Dropped["OBJE"]++
			continue
		}
		if opts.keepOriginalData() {
			out.Children = append(out.Children, cloneNode(child))
			switch child.Tag {
			case "GRAD":
				converted := convertGRAD(child, stats)
				if !nodesEqual(converted, child) && !containsNode(n.Children, converted) {
					out.Children = append(out.Children, converted)
				}
			case "_MTTAG":
				if converted := convertMTTag(child, opts); converted != nil &&
					!containsNode(n.Children, converted) {
					out.Children = append(out.Children, converted)
					stats.Converted["_MTTAG→"+converted.Tag]++
				}
			default:
				converted := preserveAndConvertSOURCitations(child, stats)
				if !nodesEqual(converted, child) {
					out.Children[len(out.Children)-1] = converted
				}
			}
			continue
		}
		if dropTags[child.Tag] {
			stats.Dropped[child.Tag]++
			continue
		}
		switch child.Tag {
		case "GRAD":
			out.Children = append(out.Children, convertGRAD(child, stats))
		case "_MTTAG":
			if fact := convertMTTag(child, opts); fact != nil {
				out.Children = append(out.Children, fact)
				stats.Converted["_MTTAG→"+fact.Tag]++
			} else {
				stats.Dropped["_MTTAG"]++
			}
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

// convertMTTag converts a 1 _MTTAG @T#@ reference on an INDI to FACT or EVEN.
func convertMTTag(n *gedcom.Node, opts Options) *gedcom.Node {
	info, ok := opts.MTTagMap[n.Value]
	if !ok || info.Name == "" {
		return nil
	}
	fact := &gedcom.Node{Level: n.Level, Tag: opts.customTagGEDCOMTag(), Value: info.Name}
	if catName := opts.MTCatMap[info.CatXRef].Name; catName != "" {
		fact.Children = append(fact.Children,
			&gedcom.Node{Level: n.Level + 1, Tag: "TYPE", Value: catName},
		)
	}
	if info.Note != "" {
		fact.Children = append(fact.Children,
			&gedcom.Node{Level: n.Level + 1, Tag: "NOTE", Value: info.Note},
		)
	}
	return fact
}

// convertFAM processes a family record.
func convertFAM(n *gedcom.Node, stats *Stats, opts Options) *gedcom.Node {
	out := shallowCopy(n)
	for _, child := range n.Children {
		if opts.NoMedia && child.Tag == "OBJE" {
			stats.Dropped["OBJE"]++
			continue
		}
		if opts.keepOriginalData() {
			converted := preserveAndConvertSOURCitations(child, stats)
			if child.Tag == "CHIL" && !opts.NoFRel {
				if !hasChildTag(converted, "_FREL") {
					converted.Children = append(converted.Children,
						&gedcom.Node{Level: child.Level + 1, Tag: "_FREL", Value: "Natural"})
				}
				if !hasChildTag(converted, "_MREL") {
					converted.Children = append(converted.Children,
						&gedcom.Node{Level: child.Level + 1, Tag: "_MREL", Value: "Natural"})
				}
				if !nodesEqual(converted, child) {
					stats.Converted["CHIL→_FREL/_MREL"]++
				}
			}
			out.Children = append(out.Children, converted)
			continue
		}
		if dropTags[child.Tag] {
			stats.Dropped[child.Tag]++
			continue
		}
		if child.Tag == "CHIL" {
			converted := cloneNode(child)
			if !opts.NoFRel {
				added := false
				if !hasChildTag(converted, "_FREL") {
					converted.Children = append(converted.Children,
						&gedcom.Node{Level: child.Level + 1, Tag: "_FREL", Value: "Natural"})
					added = true
				}
				if !hasChildTag(converted, "_MREL") {
					converted.Children = append(converted.Children,
						&gedcom.Node{Level: child.Level + 1, Tag: "_MREL", Value: "Natural"})
					added = true
				}
				if added {
					stats.Converted["CHIL→_FREL/_MREL"]++
				}
			}
			out.Children = append(out.Children, converted)
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

func preserveOBJE(n *gedcom.Node, stats *Stats) *gedcom.Node {
	out := shallowCopy(n)
	for _, child := range n.Children {
		out.Children = append(out.Children, cloneNode(child))
		if child.Tag == "DATE" {
			converted := cloneNode(child)
			converted.Tag = "_DATE"
			if !containsNode(n.Children, converted) {
				out.Children = append(out.Children, converted)
				stats.Converted["OBJE DATE→_DATE"]++
			}
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

// preserveAndConvertSOURCitations retains every original node while adding
// idempotent _LINK and NOTE siblings for DATA/WWW citation URLs.
func preserveAndConvertSOURCitations(n *gedcom.Node, stats *Stats) *gedcom.Node {
	out := shallowCopy(n)
	for _, child := range n.Children {
		out.Children = append(out.Children, preserveAndConvertSOURCitations(child, stats))
		if child.Tag != "DATA" {
			continue
		}
		url := extractWWW(child)
		if url == "" {
			continue
		}
		link := &gedcom.Node{Level: child.Level, Tag: "_LINK", Value: url}
		note := &gedcom.Node{Level: child.Level, Tag: "NOTE", Value: url}
		added := false
		if !containsNode(n.Children, link) {
			out.Children = append(out.Children, link)
			added = true
		}
		if !containsNode(n.Children, note) {
			out.Children = append(out.Children, note)
			added = true
		}
		if added {
			stats.Converted["DATA/WWW→_LINK"]++
		}
	}
	return out
}

func hasChildTag(n *gedcom.Node, tag string) bool {
	for _, child := range n.Children {
		if child.Tag == tag {
			return true
		}
	}
	return false
}

func containsNode(nodes []*gedcom.Node, want *gedcom.Node) bool {
	for _, node := range nodes {
		if nodesEqual(node, want) {
			return true
		}
	}
	return false
}

func nodesEqual(a, b *gedcom.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Level != b.Level || a.XRef != b.XRef || a.Tag != b.Tag || a.Value != b.Value ||
		len(a.Children) != len(b.Children) {
		return false
	}
	for i := range a.Children {
		if !nodesEqual(a.Children[i], b.Children[i]) {
			return false
		}
	}
	return true
}

func cloneNode(n *gedcom.Node) *gedcom.Node {
	if n == nil {
		return nil
	}
	out := shallowCopy(n)
	for _, child := range n.Children {
		out.Children = append(out.Children, cloneNode(child))
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

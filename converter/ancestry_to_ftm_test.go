package converter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ajkessel/ancestry-tag-converter/gedcom"
)

const sampleAncestry = `0 HEAD
1 SOUR Ancestry.com Family Trees
0 @I1@ INDI
1 NAME Jane /Doe/
1 _APID internal-id
1 _MTTAG @T1@
1 GRAD
2 NOTE Example High School
1 BIRT
2 DATE 1900
2 SOUR @S1@
3 DATA
4 WWW https://example.test/source
0 @F1@ FAM
1 CHIL @I1@
0 @O1@ OBJE
1 DATE 1 JAN 2000
1 _OID object-id
0 @T1@ _MTTAG
1 NAME DNA Match
1 _MTCAT @C1@
1 NOTE Custom note
0 @C1@ _MTCAT
1 NAME Research
0 TRLR
`

func TestKeepConversionIsIdempotent(t *testing.T) {
	opts := scanOptions(t, sampleAncestry, OriginalDataKeep, CustomTagFact)
	once := convertText(t, sampleAncestry, opts)
	twice := convertText(t, once, scanOptions(t, once, OriginalDataKeep, CustomTagFact))

	if once != twice {
		t.Fatalf("second conversion changed output\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
	for _, want := range []string{
		"1 _APID internal-id",
		"1 _MTTAG @T1@",
		"1 FACT DNA Match",
		"2 TYPE Research",
		"3 DATA",
		"4 WWW https://example.test/source",
		"3 _LINK https://example.test/source",
		"1 DATE 1 JAN 2000",
		"1 _DATE 1 JAN 2000",
		"0 @T1@ _MTTAG",
		"0 @C1@ _MTCAT",
	} {
		if !strings.Contains(once, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestDiscardConversionIsIdempotent(t *testing.T) {
	opts := scanOptions(t, sampleAncestry, OriginalDataDiscard, CustomTagFact)
	once := convertText(t, sampleAncestry, opts)
	twice := convertText(t, once, scanOptions(t, once, OriginalDataDiscard, CustomTagFact))

	if once != twice {
		t.Fatalf("second conversion changed output\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
	for _, unwanted := range []string{"_APID", "0 @T1@ _MTTAG", "0 @C1@ _MTCAT", "4 WWW"} {
		if strings.Contains(once, unwanted) {
			t.Errorf("discard output unexpectedly contains %q", unwanted)
		}
	}
	if count := strings.Count(once, "2 _FREL Natural"); count != 1 {
		t.Errorf("got %d _FREL records, want 1", count)
	}
}

func TestCustomTagsCanBecomeEvents(t *testing.T) {
	opts := scanOptions(t, sampleAncestry, OriginalDataKeep, CustomTagEvent)
	output := convertText(t, sampleAncestry, opts)

	if !strings.Contains(output, "1 EVEN DNA Match\n2 TYPE Research\n2 NOTE Custom note\n") {
		t.Fatalf("EVENT option did not emit the expected EVEN record:\n%s", output)
	}
	if strings.Contains(output, "1 FACT DNA Match") {
		t.Fatal("EVENT option also emitted a FACT record")
	}
}

func TestNoMediaOverridesKeepMode(t *testing.T) {
	input := strings.Replace(sampleAncestry, "1 _MTTAG @T1@", "1 OBJE @O1@\n1 _MTTAG @T1@", 1)
	opts := scanOptions(t, input, OriginalDataKeep, CustomTagFact)
	opts.NoMedia = true
	output := convertText(t, input, opts)

	if strings.Contains(output, " OBJE") {
		t.Fatalf("no-media retained an OBJE record or reference:\n%s", output)
	}
}

func TestMergeCustomTagDefinitionsIsIdempotentAndCollisionSafe(t *testing.T) {
	opts := scanOptions(t, sampleAncestry, OriginalDataKeep, CustomTagFact)
	baseText := `0 HEAD
1 SOUR Family Tree Maker
0 @I9@ INDI
1 NAME Jane /Doe/
1 BIRT
2 DATE 1900
0 @T1@ NOTE
1 CONT unrelated collision
0 TRLR
`
	base := parseIndexed(t, baseText)
	src := findRecord(t, sampleAncestry, "INDI")
	converted := Convert(src, NewStats(), opts)

	plan := PrepareCustomTagMerge(base, opts)
	plan.RewriteAndMarkINDI(converted)
	MergeINDI(base.IndiByKey[IndividualKey(converted)], converted, NewStats())
	plan.AppendDefinitions()

	merged := writeRecords(t, base.Records)
	if !strings.Contains(merged, "1 _MTTAG @T1_ATC1@") {
		t.Fatalf("custom-tag reference was not remapped:\n%s", merged)
	}
	if !strings.Contains(merged, "0 @T1_ATC1@ _MTTAG") ||
		!strings.Contains(merged, "0 @C1@ _MTCAT") {
		t.Fatalf("referenced definitions were not added:\n%s", merged)
	}

	reloaded := parseIndexed(t, merged)
	recordCount := len(reloaded.Records)
	convertedAgain := Convert(findRecord(t, sampleAncestry, "INDI"), NewStats(), opts)
	planAgain := PrepareCustomTagMerge(reloaded, opts)
	planAgain.RewriteAndMarkINDI(convertedAgain)
	MergeINDI(reloaded.IndiByKey[IndividualKey(convertedAgain)], convertedAgain, NewStats())
	planAgain.AppendDefinitions()

	mergedAgain := writeRecords(t, reloaded.Records)
	if merged != mergedAgain {
		t.Fatalf("repeated merge changed output\n--- once ---\n%s\n--- twice ---\n%s", merged, mergedAgain)
	}
	if len(reloaded.Records) != recordCount {
		t.Fatalf("repeated merge added records: got %d, want %d", len(reloaded.Records), recordCount)
	}
}

func scanOptions(t *testing.T, input string, mode OriginalDataMode, record CustomTagRecord) Options {
	t.Helper()
	tags, cats, err := ScanMTTagsFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	return Options{
		OriginalData:    mode,
		CustomTagRecord: record,
		MTTagMap:        tags,
		MTCatMap:        cats,
	}
}

func convertText(t *testing.T, input string, opts Options) string {
	t.Helper()
	parser := gedcom.NewParser(strings.NewReader(input))
	var records []*gedcom.Node
	stats := NewStats()
	for {
		record := parser.Next()
		if record == nil {
			break
		}
		if converted := Convert(record, stats, opts); converted != nil {
			records = append(records, converted)
		}
	}
	records = append(records, &gedcom.Node{Level: 0, Tag: "TRLR"})
	return writeRecords(t, records)
}

func parseIndexed(t *testing.T, input string) *IndexedGEDCOM {
	t.Helper()
	base, err := LoadAndIndexFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	return base
}

func findRecord(t *testing.T, input, tag string) *gedcom.Node {
	t.Helper()
	parser := gedcom.NewParser(strings.NewReader(input))
	for {
		record := parser.Next()
		if record == nil {
			t.Fatalf("record %s not found", tag)
		}
		if record.Tag == tag {
			return record
		}
	}
}

func writeRecords(t *testing.T, records []*gedcom.Node) string {
	t.Helper()
	var out bytes.Buffer
	for _, record := range records {
		if err := gedcom.WriteRecord(&out, record); err != nil {
			t.Fatal(err)
		}
	}
	return out.String()
}

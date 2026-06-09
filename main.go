package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ajkessel/ancestry-tag-converter/converter"
	"github.com/ajkessel/ancestry-tag-converter/gedcom"
)

func main() {
	noFRel := flag.Bool("no-frel", false, "don't add _FREL/_MREL Natural to CHIL records")
	noMedia := flag.Bool("no-media", false, "drop all OBJE records and inline OBJE references")
	mergeBase := flag.String("merge", "", "FTM base GEDCOM to merge converted records into (preserves all base data)")
	originalData := flag.String("original-data", "keep", "original input data: keep or discard")
	customTags := flag.String("custom-tags", "fact", "custom tag output record: fact or event")
	verbose := flag.Bool("verbose", false, "print conversion statistics to stderr")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: ancestry-tag-converter [flags] input.ged output.ged")
		fmt.Fprintln(os.Stderr, "       ancestry-tag-converter [flags] --merge ftm-base.ged ancestry.ged output.ged")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *originalData != string(converter.OriginalDataKeep) &&
		*originalData != string(converter.OriginalDataDiscard) {
		fmt.Fprintln(os.Stderr, "error: --original-data must be keep or discard")
		os.Exit(1)
	}
	if *customTags != string(converter.CustomTagFact) &&
		*customTags != string(converter.CustomTagEvent) {
		fmt.Fprintln(os.Stderr, "error: --custom-tags must be fact or event")
		os.Exit(1)
	}

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(1)
	}
	inputPath := flag.Arg(0)
	outputPath := flag.Arg(1)

	// Auto-detect argument order for --merge: if the user passed the Ancestry
	// file as the --merge base and the FTM file as the positional input, swap
	// them automatically rather than silently producing wrong output.
	if *mergeBase != "" {
		ancestryInMerge := isAncestryFile(*mergeBase)
		ancestryInInput := isAncestryFile(inputPath)
		if ancestryInMerge && !ancestryInInput {
			fmt.Fprintf(os.Stderr, "note: swapping argument order — %s is the Ancestry file, %s is the FTM base\n", *mergeBase, inputPath)
			*mergeBase, inputPath = inputPath, *mergeBase
		}
	}

	inputSize := fileSize(inputPath)

	// First pass: collect _MTTAG and _MTCAT definitions for name lookup.
	scanFile, err := os.Open(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening input: %v\n", err)
		os.Exit(1)
	}
	pb1 := newProgressBar("Scanning tags…", inputSize)
	mttagMap, mtcatMap, err := converter.ScanMTTagsFromReader(&progressReader{r: scanFile, bar: pb1})
	pb1.finish()
	scanFile.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error scanning MT tags: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(outputPath); err == nil {
		fmt.Fprintf(os.Stderr, "%s already exists. Overwrite? [y/N] ", outputPath)
		var answer string
		fmt.Fscan(os.Stdin, &answer)
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			os.Exit(1)
		}
	}

	out, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating output: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	bw := bufio.NewWriterSize(out, 4*1024*1024) // 4MB write buffer

	opts := converter.Options{
		NoFRel:          *noFRel,
		NoMedia:         *noMedia,
		OriginalData:    converter.OriginalDataMode(*originalData),
		CustomTagRecord: converter.CustomTagRecord(*customTags),
		MTTagMap:        mttagMap,
		MTCatMap:        mtcatMap,
	}
	stats := converter.NewStats()
	start := time.Now()
	var unmatchedKeys []string

	if *mergeBase != "" {
		// Merge mode: load FTM base into memory, merge converted Ancestry events in.
		baseFile, err := os.Open(*mergeBase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening merge base: %v\n", err)
			os.Exit(1)
		}
		pb2 := newProgressBar("Loading base…", fileSize(*mergeBase))
		base, err := converter.LoadAndIndexFromReader(&progressReader{r: baseFile, bar: pb2})
		pb2.finish()
		baseFile.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading merge base: %v\n", err)
			os.Exit(1)
		}
		for _, w := range base.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
		customTagPlan := converter.PrepareCustomTagMerge(base, opts)

		in, err := os.Open(inputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening input: %v\n", err)
			os.Exit(1)
		}
		pb3 := newProgressBar("Converting…", inputSize)
		matched, unmatched := 0, 0
		parser := gedcom.NewParser(&progressReader{r: in, bar: pb3})
		for {
			rec := parser.Next()
			if rec == nil {
				break
			}
			converted := converter.Convert(rec, stats, opts)
			if converted == nil || converted.Tag != "INDI" {
				continue
			}
			key := converter.IndividualKey(converted)
			if baseIndi, ok := base.IndiByKey[key]; ok {
				customTagPlan.RewriteAndMarkINDI(converted)
				converter.MergeINDI(baseIndi, converted, stats)
				matched++
			} else if baseIndi, ambiguous := base.FuzzyMatchINDI(converted); baseIndi != nil {
				if ambiguous {
					fmt.Fprintf(os.Stderr, "warning: ambiguous fuzzy match for %s (key=%q); using first candidate\n", converted.XRef, key)
				}
				customTagPlan.RewriteAndMarkINDI(converted)
				converter.MergeINDI(baseIndi, converted, stats)
				matched++
			} else {
				unmatched++
				if *verbose {
					unmatchedKeys = append(unmatchedKeys, fmt.Sprintf("  unmatched: %s (key=%q)", converted.XRef, key))
				}
			}
		}
		pb3.finish()
		in.Close()
		customTagPlan.AppendDefinitions()

		// Write base GEDCOM (TRLR is included in base.Records)
		pb4 := newProgressBar("Writing…", int64(len(base.Records)))
		for _, rec := range base.Records {
			if err := gedcom.WriteRecord(bw, rec); err != nil {
				fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
				os.Exit(1)
			}
			pb4.update(1)
		}
		pb4.finish()
		stats.Converted["merge:matched"] = matched
		stats.Converted["merge:unmatched"] = unmatched
	} else {
		// Conversion-only mode: convert Ancestry GEDCOM, write directly.
		in, err := os.Open(inputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening input: %v\n", err)
			os.Exit(1)
		}
		pb2 := newProgressBar("Converting…", inputSize)
		parser := gedcom.NewParser(&progressReader{r: in, bar: pb2})
		for {
			rec := parser.Next()
			if rec == nil {
				break
			}
			converted := converter.Convert(rec, stats, opts)
			if converted == nil {
				continue
			}
			if err := gedcom.WriteRecord(bw, converted); err != nil {
				fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
				os.Exit(1)
			}
		}
		pb2.finish()
		in.Close()
		// GEDCOM trailer (conversion mode only; merge mode gets it from base)
		fmt.Fprintln(bw, "0 TRLR")
	}

	// Flush file before any verbose output so a broken pipe can't truncate the file.
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "error flushing output: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(start)
	if *mergeBase != "" {
		matched := stats.Converted["merge:matched"]
		unmatched := stats.Converted["merge:unmatched"]
		skipped := stats.Converted["merge:skipped"]
		fmt.Fprintf(os.Stderr, "Merge complete in %s: %d matched, %d unmatched, %d duplicate events skipped\n",
			elapsed.Round(time.Millisecond), matched, unmatched, skipped)
	} else {
		total := 0
		for _, n := range stats.Records {
			total += n
		}
		fmt.Fprintf(os.Stderr, "Conversion complete in %s: %d records processed\n",
			elapsed.Round(time.Millisecond), total)
	}

	if *verbose {
		fmt.Fprintln(os.Stderr)

		if len(unmatchedKeys) > 0 {
			fmt.Fprintln(os.Stderr, "Unmatched individuals:")
			for _, k := range unmatchedKeys {
				fmt.Fprintln(os.Stderr, k)
			}
			fmt.Fprintln(os.Stderr)
		}

		fmt.Fprintln(os.Stderr, "Records processed:")
		printSortedMap(os.Stderr, stats.Records)

		fmt.Fprintln(os.Stderr, "\nTags dropped:")
		printSortedMap(os.Stderr, stats.Dropped)

		fmt.Fprintln(os.Stderr, "\nConversions applied:")
		printSortedMap(os.Stderr, stats.Converted)
	}
}

// isAncestryFile returns true if the file's HEAD identifies it as an Ancestry
// export. It reads only the first 20 lines, so it's fast even on large files.
func isAncestryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for i := 0; i < 20 && sc.Scan(); i++ {
		if strings.Contains(sc.Text(), "Ancestry") {
			return true
		}
	}
	return false
}

func printSortedMap(f *os.File, m map[string]int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(f, "  %-25s %d\n", k, m[k])
	}
}

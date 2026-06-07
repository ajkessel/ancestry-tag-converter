package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/ajkessel/ancestry-tag-converter/converter"
	"github.com/ajkessel/ancestry-tag-converter/gedcom"
)

func main() {
	noFRel := flag.Bool("no-frel", false, "don't add _FREL/_MREL Natural to CHIL records")
	noMedia := flag.Bool("no-media", false, "drop all OBJE records and inline OBJE references")
	verbose := flag.Bool("verbose", false, "print conversion statistics to stderr")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: ancestry-to-ftm [flags] input.ged output.ged")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(1)
	}
	inputPath := flag.Arg(0)
	outputPath := flag.Arg(1)

	in, err := os.Open(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening input: %v\n", err)
		os.Exit(1)
	}
	defer in.Close()

	out, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating output: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	bw := bufio.NewWriterSize(out, 4*1024*1024) // 4MB write buffer

	opts := converter.Options{
		NoFRel:  *noFRel,
		NoMedia: *noMedia,
	}
	stats := converter.NewStats()
	parser := gedcom.NewParser(in)

	start := time.Now()
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

	// GEDCOM trailer
	fmt.Fprintln(bw, "0 TRLR")

	if err := bw.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "error flushing output: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		elapsed := time.Since(start)
		fmt.Fprintf(os.Stderr, "\n=== Conversion complete in %s ===\n\n", elapsed.Round(time.Millisecond))

		fmt.Fprintln(os.Stderr, "Records processed:")
		printSortedMap(os.Stderr, stats.Records)

		fmt.Fprintln(os.Stderr, "\nTags dropped:")
		printSortedMap(os.Stderr, stats.Dropped)

		fmt.Fprintln(os.Stderr, "\nConversions applied:")
		printSortedMap(os.Stderr, stats.Converted)
	}
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

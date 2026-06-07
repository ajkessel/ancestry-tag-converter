# Technical Reference

## Repository layout

```
ancestry-tag-converter/
├── main.go                        CLI entry point, flag parsing, orchestration
├── progress.go                    Terminal progress bar (CLI-only)
├── gedcom/
│   ├── record.go                  Node struct and line parser
│   ├── parser.go                  Streaming record-at-a-time parser
│   └── writer.go                  GEDCOM writer with UTF-8-safe line splitting
├── converter/
│   ├── ancestry_to_ftm.go         All conversion rules + first-pass MT tag scanner
│   └── merge.go                   Merge index, individual matching, event deduplication
└── cmd/
    └── ancestry-tag-converter-gui/
        └── main.go                Fyne GUI, replicates CLI orchestration with GUI feedback
```

## GEDCOM format

GEDCOM is a line-oriented hierarchical text format. Each line is:

```
LEVEL [XREF] TAG [VALUE]
```

- `LEVEL` is a non-negative integer. Level 0 begins a new top-level record.
- `XREF` (optional, only on level-0 lines) is a cross-reference ID like `@I42@`.
- `TAG` is the record or field type.
- `VALUE` is free text, optional.

The hierarchy is implicit: a line at level N is a child of the most recent preceding line at level N−1.

Example individual record:

```
0 @I42@ INDI
1 NAME John /Smith/
1 BIRT
2 DATE 15 Jun 1952
2 PLAC Boston, Massachusetts
1 SOUR @S101@
2 DATA
3 WWW https://www.ancestry.com/...
```

Long values may be continued across lines using `CONC` at level+1.

## Data model

The parser materializes each level-0 record into a tree of `Node` structs:

```go
// gedcom/record.go
type Node struct {
    Level    int
    XRef     string     // only on level-0 nodes, e.g. "@I42@"
    Tag      string
    Value    string
    Children []*Node
}
```

The tree is processed in memory, transformed, and written back to disk. Because records are processed one at a time, peak memory is bounded by the size of the largest record (typically a few KB), not the file size.

## Parser (`gedcom/parser.go`)

`NewParser(r io.Reader)` returns a streaming parser. Each call to `Next()` reads lines until the next level-0 boundary and returns the completed record as a `*Node` tree, or `nil` at EOF.

Implementation details:

- Uses `bufio.Scanner` with a 1 MB buffer (accommodates long continuation lines).
- Strips the UTF-8 BOM (`\xef\xbb\xbf`) from the first line if present.
- Maintains a single `pending` node to buffer the lookahead level-0 line that signals the end of the current record.

## Writer (`gedcom/writer.go`)

`WriteRecord(w io.Writer, n *Node)` serialises a `Node` tree recursively.

Long values are split at 255 **runes** (not bytes) using `CONC` continuation lines at level+1. Slicing at rune boundaries avoids splitting multi-byte UTF-8 sequences, which would produce an invalid file.

## Two-pass processing

`_MTTAG` records are Ancestry's DNA/matching feature. Each individual's `_MTTAG` children reference a level-0 `_MTTAG` record (via XRef) that holds the human-readable name and category. Because the definitions typically appear near the end of the file and the references appear earlier, a single forward pass cannot resolve them.

**Pass 1 — `converter.ScanMTTags(path)`**

Opens the file, parses every record, and builds two maps:

- `MTTagMap map[string]MTTagInfo` — XRef → `{Name, CatXRef, Note}`
- `MTCatMap map[string]string` — XRef → category name

**Pass 2 — main conversion**

`converter.Convert(node, stats, opts)` processes each record. When it encounters a `1 _MTTAG @T…@` child on an `INDI`, it looks up the XRef in `MTTagMap` and emits:

```
1 FACT  <human-readable name>
2 TYPE  <category name>
2 NOTE  <note>
```

Both passes use the same streaming parser, so the file is read twice sequentially; memory usage is bounded.

## Conversion rules

### Tags dropped entirely (with all children)

| Tag | Context |
|-----|---------|
| `_APID` | Ancestry record IDs on source citations and names |
| `_HPID`, `_WPID` | Ancestry husband/wife picture IDs on `MARR` |
| `_OID` | Object UUID on `OBJE` |
| `_META` | XML metadata blob on `OBJE` |
| `_CREA` | Creation timestamp on `OBJE` |
| `_USER`, `_ENCR` | Encrypted user ID |
| `_CLON` | Clone-tracking container |
| `_ORIG` | Origin indicator |
| `_MSER` | Media source container |
| `_ATL` | "Attached to living" flag |
| `_PRIM` | Primary media flag |
| `_STYPE`, `_WDTH`, `_HGHT`, `_SIZE`, `_MTYPE` | Image metadata under `FILE/FORM` |
| `_TREE`, `_ENV` | Ancestry tree/environment in `HEAD SOUR` |
| `_WLNK` | Ancestry web link tag |

`_MTTAG` and `_MTCAT` level-0 records are also dropped (after the first pass extracts their names).

### Tags converted

**Source citation URLs (`DATA/WWW` → `_LINK` + `NOTE`)**

Ancestry stores citation URLs as:
```
3 DATA
4 WWW https://…
```
FTM expects:
```
3 _LINK https://…
3 NOTE  https://…
```
Rule: when a `SOUR` citation contains a `DATA` child with a `WWW` grandchild, the URL is promoted to `_LINK` and `NOTE` siblings and the `DATA` node is dropped. Non-URL children of `DATA` are kept.

**Child relationship types (`CHIL` → `_FREL`/`_MREL`)**

FTM requires relationship-type tags after each `CHIL` pointer in `FAM` records:
```
1 CHIL @I5@
2 _FREL Natural
2 _MREL Natural
```
Ancestry omits these. The converter appends both after every `CHIL` (unless `--no-frel` is set). "Natural" is used as the default since Ancestry exports carry no relationship-type information.

**Media dates (`DATE` → `_DATE` in `OBJE`)**

Inside `OBJE` records, standard `DATE` tags are renamed to `_DATE`, which is FTM's convention for media-item dates.

**Graduation school names (`GRAD/NOTE` → inline value)**

Ancestry stores school names as:
```
1 GRAD
2 NOTE Champlain Valley Union High School
```
FTM stores them inline:
```
1 GRAD Champlain Valley Union High School
```
The converter checks whether the `NOTE` value "looks like a school name" (short, starts with an uppercase letter, no mid-sentence periods) and, if so, moves it to the `GRAD` value.

**`HEAD` cleanup**

The `HEAD SOUR` block is replaced with a minimal identifier (`ancestry-tag-converter`). All Ancestry corporate/version fields are dropped.

**`OBJE` records**

Ancestry `OBJE` records contain many internal fields. The converter keeps only: `FILE` (with `FORM/TYPE` child), `TITL`, `NOTE`, `_DATE`, `_TEXT`, `_DSCR`, `PLAC`. All other children are dropped. The `FILE` value itself (the local path) is preserved even though Ancestry exports don't include actual file paths — this leaves the skeleton in place for manual re-linking in FTM.

## Merge algorithm (`converter/merge.go`)

### Indexing

`LoadAndIndex(path)` parses the entire FTM base file into memory and builds:

- `ByXRef map[string]*Node` — XRef → record (for lookup by ID)
- `IndiByKey map[string]*Node` — match key → INDI record (for cross-file matching)

### Individual match key

Because Ancestry and FTM use entirely different XRef schemes, cross-file matching uses a content-derived key:

```
key = normalizedName + "|" + birthYear
```

where:
- `normalizedName` strips `/` surname delimiters, lowercases, and collapses whitespace from the `NAME` value.
- `birthYear` is the four-digit year extracted from the `BIRT/DATE` grandchild, or absent if none exists.

If no birth year is available, the key is just the normalized name. This means individuals with common names and no birth year may collide or fail to match — see Limitations in README.md.

### Event deduplication

`buildExistingSet(indi)` returns a `map[string]struct{}` of all deduplication signatures already present in an FTM individual. `eventSignature(node)` produces the key:

```
TAG|val:VALUE|type:TYPE|date:DATE|plac:PLAC
```

All components are lowercased and whitespace-normalised. Two events with the same signature are considered duplicates and the Ancestry version is skipped.

**Singleton tags** (`BIRT`, `DEAT`, `CHR`, `BAPM`, `BURI`, `FCOM`, `CONF`) are only allowed once per individual. If the FTM record already has one, the Ancestry version is skipped regardless of its content.

**Tags never merged** (always skipped): `NAME`, `SEX`, `FAMC`, `FAMS`, `REFN`, `RIN`, `CHAN`, `OBJE`, `SOUR`.

### Merge loop

```
for each Ancestry INDI (converted):
    key = IndividualKey(indi)
    if base.IndiByKey[key] exists:
        MergeINDI(baseIndi, ancestryIndi)   // append non-duplicate children
        matched++
    else:
        unmatched++
```

After the loop, `base.Records` (including INDI records now augmented with new events) is written to the output file. The FTM `TRLR` record comes from `base.Records`; no second trailer is written.

## Progress tracking

### CLI (`progress.go`)

`progressBar` tracks `done` and `total` bytes. A `progressReader` wraps any `io.Reader` and calls `bar.update(n)` on each `Read`. Updates are throttled to 0.5% increments to limit `fmt.Fprintf` calls.

When `os.Stderr` is a character device (`fi.Mode()&os.ModeCharDevice != 0`), the bar renders in-place using `\r`. Otherwise, each phase prints a plain label + "done" line (suitable for redirected output or CI).

Four phases in merge mode:

| Phase | Label | Progress source |
|-------|-------|-----------------|
| 1 | `Scanning tags…` | bytes read from Ancestry file (first pass) |
| 2 | `Loading base…` | bytes read from FTM base file |
| 3 | `Converting…` | bytes read from Ancestry file (second pass) |
| 4 | `Writing…` | records written out of `len(base.Records)` |

In convert-only mode, phases 2 and 4 are omitted and the fractions are redistributed.

### GUI (`cmd/ancestry-tag-converter-gui/main.go`)

`phaseBar` maps each phase to a sub-range of a `widget.ProgressBar` value (0.0–1.0). A `phaseReader` wraps the `io.Reader` and calls `phaseBar.add(n)` on each read, which calls `bar.SetValue(base + frac*span)`.

Fyne widget updates are thread-safe; the conversion goroutine updates the progress bar directly without dispatching to the main thread.

Phase allocations in merge mode:

| Phase | Bar range |
|-------|-----------|
| Scan tags | 0%–20% |
| Load base | 20%–50% |
| Convert | 50%–85% |
| Write | 85%–100% |

## Building

### CLI

```bash
go build -o dist/ancestry-tag-converter .
```

No CGO required. Pure Go.

### GUI

```bash
go build -o dist/ancestry-tag-converter-gui ./cmd/ancestry-tag-converter-gui/
```

Requires CGO and platform graphics libraries (Fyne uses OpenGL on Linux/Windows, Metal on macOS via GLFW).

**Linux** — install X11/OpenGL development headers:
```bash
sudo apt install libgl1-mesa-dev libxxf86vm-dev libxrandr-dev \
                 libxi-dev libxcursor-dev libxinerama-dev
```

**macOS** — Xcode CLT only:
```bash
xcode-select --install
```

**Windows** — install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or use MSYS2 with `mingw-w64-x86_64-gcc`.

### Cross-compiling the GUI

Cross-compiling a Fyne application requires a cross-compile C toolchain. The [fyne-cross](https://github.com/fyne-io/fyne-cross) tool automates this via Docker:

```bash
go install github.com/fyne-io/fyne-cross@latest
fyne-cross linux   -arch=amd64
fyne-cross windows -arch=amd64
fyne-cross darwin  -arch=amd64,arm64
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `fyne.io/fyne/v2` | GUI framework (GUI binary only) |

The CLI has zero external dependencies. All other entries in `go.sum` are transitive Fyne dependencies.

## Known edge cases

**UTF-8 BOM** — Ancestry exports sometimes include a UTF-8 byte-order mark. The parser strips it from the first line.

**Long lines** — The GEDCOM spec limits lines to 255 characters. The writer enforces this by splitting long values into `CONC` continuation lines, slicing at rune boundaries to avoid corrupting multi-byte sequences.

**SIGPIPE** — When the user pipes output (e.g., `ancestry-tag-converter … | head`), a broken pipe kills the process. The CLI flushes the output `bufio.Writer` before printing any verbose statistics to stderr, ensuring the file is complete even if the pipe is broken by the reader.

**Argument auto-detection** — `isAncestryFile(path)` scans the first 20 lines of a file for the string `"Ancestry"` in the `HEAD`. If the `--merge` argument points to the Ancestry file and the positional argument points to the FTM base (reversed), the files are swapped automatically with a printed notice. The GUI applies the same check before starting conversion.

# ancestry-to-ftm

Converts [Ancestry.com](https://www.ancestry.com) GEDCOM exports to [Family Tree Maker](https://www.mackiev.com/ftm/) (FTM)-compatible GEDCOM files, and optionally merges the converted records into an existing FTM tree.

## Background

Both Ancestry and FTM use the [GEDCOM 5.5.1](https://gedcom.io/specifications/FamilySearchGEDCOMv7.html) standard as their exchange format, but each adds dozens of proprietary custom tags the other doesn't understand. An Ancestry export dropped into FTM will import with thousands of unrecognized `_APID`, `_OID`, `_MTTAG`, and similar tags cluttering every record. Conversely, FTM conventions like `_FREL`/`_MREL` on child relationships are absent from Ancestry exports.

This tool bridges that gap: it strips Ancestry-internal tags, converts Ancestry conventions to FTM conventions, and can selectively merge new data from an Ancestry export into your existing FTM tree without duplicating events you already have.

## Features

- Strips 20+ Ancestry-internal tags (`_APID`, `_OID`, `_CLON`, `_META`, `_WLNK`, …)
- Converts source citation URLs (`DATA/WWW`) to FTM's `_LINK` + `NOTE` format
- Adds `_FREL Natural` / `_MREL Natural` after each `CHIL` record in families
- Converts media dates (`DATE` → `_DATE` inside `OBJE` records)
- Converts graduation school names from `NOTE` children to inline `GRAD` values
- Converts `_MTTAG` DNA/matching tags to human-readable `FACT` entries (two-pass lookup)
- Merge mode: preserves all data from an existing FTM file, adding only new events from Ancestry without duplicating anything
- Automatic argument-order detection (swaps Ancestry/FTM files if passed in the wrong order)
- Available as both a command-line tool and a native GUI

## Installation

### Prerequisites

**CLI:** Go 1.21 or later. No external dependencies.

**GUI:** Go 1.21 or later plus Fyne's native dependencies:

| Platform | Required packages |
|----------|------------------|
| Linux | `libgl1-mesa-dev libxxf86vm-dev libxrandr-dev libxi-dev libxcursor-dev libxinerama-dev` (or distro equivalents) |
| macOS | Xcode Command Line Tools (`xcode-select --install`) |
| Windows | A C compiler such as [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or MSYS2 |

### Build

```bash
# Clone
git clone https://github.com/ajkessel/ancestry-tag-converter
cd ancestry-tag-converter

# CLI tool
go build -o dist/ancestry-to-ftm .

# GUI (optional)
go build -o dist/ancestry-to-ftm-gui ./cmd/ancestry-to-ftm-gui/
```

## CLI Usage

```
ancestry-to-ftm [flags] ancestry.ged output.ged
ancestry-to-ftm [flags] --merge ftm-base.ged ancestry.ged output.ged
```

### Flags

| Flag | Description |
|------|-------------|
| `--merge ftm-base.ged` | Merge converted Ancestry records into an existing FTM file. All FTM data is preserved; only new events are added. |
| `--no-frel` | Don't add `_FREL`/`_MREL Natural` to child records. |
| `--no-media` | Drop all `OBJE` media records and inline media references. |
| `--verbose` | Print conversion statistics to stderr after completion. |

### Examples

**Convert only** — produce a standalone FTM-compatible GEDCOM:
```bash
ancestry-to-ftm MyTree.ged MyTree_ftm.ged
```

**Convert with statistics:**
```bash
ancestry-to-ftm --verbose MyTree.ged MyTree_ftm.ged
```

**Merge into an existing FTM tree:**
```bash
ancestry-to-ftm --merge FamilyTree.ged MyTree.ged merged.ged
```
The tool automatically detects which file is the Ancestry export and which is the FTM base, so argument order doesn't matter.

**Convert without adding media records or relationship tags:**
```bash
ancestry-to-ftm --no-frel --no-media MyTree.ged MyTree_ftm.ged
```

### Progress display

When stderr is an interactive terminal, a live progress bar shows each phase:

```
Scanning tags…      [===================================] 100%
Loading base…       [===================================] 100%
Converting…         [===================================] 100%
Writing…            [===================================] 100%
```

When stderr is redirected or piped, each phase prints a simple one-line status instead.

### Output protection

The tool will not silently overwrite an existing output file. If the output path already exists, it prompts:

```
output.ged already exists. Overwrite? [y/N]
```

## GUI Usage

Launch the GUI:
```bash
./ancestry-to-ftm-gui
```

The window presents all the same options as the CLI:

1. **Ancestry file** — browse to your Ancestry GEDCOM export.
2. **Output file** — the output path is auto-suggested based on the input filename (`MyTree_ftm.ged` or `MyTree_merged.ged`). You can change it.
3. **FTM base file** — enabled when "Merge into existing FTM base file" is checked.
4. **Options** — checkboxes for skipping relationship tags and/or media records.
5. **Convert** — starts conversion. If the output file exists, a confirmation dialog appears first.

A progress bar tracks each phase. A log area at the bottom shows matched/unmatched counts and conversion summaries after the run completes.

## Merge behavior

When `--merge` is used, the tool:

1. Loads the entire FTM base file into memory and indexes all individuals by a **match key** (normalized full name + birth year).
2. Converts each individual from the Ancestry file.
3. For each Ancestry individual, finds the matching FTM individual (if any) and adds events that don't already exist in the FTM record.

**What gets merged:** Any event not already present — `FACT`, `OCCU`, `RESI`, `EDUC`, `EVEN`, custom tags, etc.

**What is never merged:** `NAME`, `SEX`, family links (`FAMC`/`FAMS`), source citations (`SOUR`), media links (`OBJE`), and structural IDs (`REFN`, `RIN`, `CHAN`).

**Singleton events** (birth, death, baptism, burial, etc.) are skipped if the FTM record already has one, regardless of the date or place.

**Deduplication** compares a canonical signature of each event: `TAG|val:…|type:…|date:…|plac:…`. Two events with the same signature are considered duplicates.

Individuals who appear in Ancestry but have no name+year match in the FTM base are counted as "unmatched" (reported with `--verbose` or in the GUI log).

## Limitations

- **Ancestry XRef IDs** (`@I102667033170@`) differ from FTM IDs (`@I21@`), so cross-file matching relies on name + birth year rather than ID. Individuals without a birth year or with ambiguous names may not match.
- **Media files** are not transferred — Ancestry GEDCOM exports don't include local file paths, only Ancestry-internal references. The structure is preserved for manual re-linking.
- **Source records** from Ancestry (`SOUR` XRefs) are not merged, as the Ancestry source registry doesn't exist in the FTM file.
- **Family matching** in merge mode only operates on individuals (`INDI`); family (`FAM`) records are not merged.

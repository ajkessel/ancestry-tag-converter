# Ancestry Tag Converter

This is a data-portability tool for genealogists who use Ancestry.

This tool converts Ancestry GEDCOM exports that contain Ancestry-unique extensions to a more GEDCOM-standard compliant version that can be understood by other applications.

This tool also optionally merges the converted records into an existing GEDCOM file (such as one exported by FTM).

## Download

Download the latest builds: 
- [Linux](https://github.com/ajkessel/ancestry-tag-converter/releases/latest/download/ancestry-tag-converter-linux.tar.gz)
- [MacOS](https://github.com/ajkessel/ancestry-tag-converter/releases/latest/download/ancestry-tag-converter-mac.dmg)
- [Windows](https://github.com/ajkessel/ancestry-tag-converter/releases/latest/download/ancestry-tag-converter-windows.zip)


## Background

This tool helps bridge the gap between Ancestry and other genealogy platforms.

Ancestry provides a "MyTreeTags" feature that allows users to apply custom tags to their trees. When an Ancestry tree is exported to GEDCOM, these tags are preserved, but not in a GEDCOM-standard compliant way. Users who sync their trees to Family Tree Maker (or import their Ancestry exported GEDCOM into FTM) lose access to these custom tags. A separate problem is that Ancestry provides no mechanism to download media (such as profile photos in your tree and other image/video/PDF content).

FTM, by contrast, allows users to export a synced Ancestry tree to a GEDCOM file with all media preserved. This GEDCOM file can then be opened in other standards-compliant applications with media like images preserved. But this file will not have any custom tags created in Ancestry.

This tool bridges that gap: it converts Ancestry conventions to FTM conventions and can selectively merge new data from an Ancestry export into your existing FTM GEDCOM export without duplicating events you already have. By default, original Ancestry data is retained alongside the converted fields; it can optionally be discarded.

This tool does not alter your existing GEDCOM files. Instead, it creates a new output/merged file.

This tool has no network interaction and does not retain any information.

## Features

- Keeps all original input data by default, with an option to discard Ancestry-internal data
- Converts source citation URLs (`DATA/WWW`) to FTM's `_LINK` + `NOTE` format
- Adds `_FREL Natural` / `_MREL Natural` after each `CHIL` record in families
- Converts media dates (`DATE` → `_DATE` inside `OBJE` records)
- Converts graduation school names from `NOTE` children to inline `GRAD` values
- Converts `_MTTAG` DNA/matching tags to human-readable `FACT` (default) or `EVEN` entries (two-pass lookup)
- Merge mode: preserves all data from an existing FTM file, adding only new events from Ancestry without duplicating anything
- Automatic argument-order detection (swaps Ancestry/FTM files if passed in the wrong order)
- Idempotent conversion and merging: repeating the same operation does not add duplicate converted data
- Available as both a command-line tool and a native GUI

## Building from source

### Prerequisites

**CLI:** Go 1.21 or later. No external dependencies.

**GUI:** Go 1.21 or later plus a C compiler and platform graphics libraries. Fyne's GLFW/OpenGL backend requires CGO.

#### Linux
```bash
sudo apt install libgl1-mesa-dev libxxf86vm-dev libxrandr-dev \
                 libxi-dev libxcursor-dev libxinerama-dev
```
(substitute your distro's equivalent package manager and names)

#### macOS
```bash
xcode-select --install
```

#### Windows

Fyne requires a GCC-compatible C compiler on `PATH`. The recommended approach is **MSYS2**:

1. Download and install MSYS2 from <https://www.msys2.org/>
2. Open the **MSYS2 MinGW 64-bit** shell and run:
   ```
   pacman -S mingw-w64-x86_64-gcc
   ```
3. Add `C:\msys64\mingw64\bin` to your Windows system `PATH`.
4. Open a new Command Prompt or PowerShell and verify:
   ```
   gcc --version
   ```

Alternatively, [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) provides a self-contained installer that adds `gcc` to `PATH` automatically.

> The CLI binary (`ancestry-tag-converter`) has **no C compiler requirement** — only the GUI needs CGO. Build the CLI with a plain `go build` if you don't need the GUI.

### Build

```bash
git clone https://github.com/ajkessel/ancestry-tag-converter
cd ancestry-tag-converter

make all     # builds both CLI and GUI into dist/
make cli     # CLI only (no C compiler required)
make gui     # GUI only
```

The `Makefile` detects Windows automatically and adds the necessary flags (`.exe` suffixes, GUI subsystem linker flag). On Windows, run these commands from an MSYS2 shell where `gcc` and `make` are on `PATH`.

## CLI Usage

```
ancestry-tag-converter [flags] ancestry.ged output.ged
ancestry-tag-converter [flags] --merge ftm-base.ged ancestry.ged output.ged
```

### Flags

| Flag | Description |
|------|-------------|
| `--merge ftm-base.ged` | Merge converted Ancestry records into an existing FTM file. All FTM data is preserved; only new events are added. |
| `--original-data keep\|discard` | Keep all original input data alongside converted fields (default), or discard data replaced/removed by conversion. |
| `--custom-tags fact\|event` | Convert custom tags to `FACT` records (default) or GEDCOM `EVEN` records. |
| `--no-frel` | Don't add `_FREL`/`_MREL Natural` to child records. |
| `--no-media` | Drop all `OBJE` media records and inline media references. |
| `--verbose` | Print conversion statistics to stderr after completion. |

### Examples

**Convert only** — produce a standalone FTM-compatible GEDCOM:
```bash
ancestry-tag-converter MyTree.ged MyTree_ftm.ged
```

**Convert with statistics:**
```bash
ancestry-tag-converter --verbose MyTree.ged MyTree_ftm.ged
```

**Discard original Ancestry-only data and create custom events:**
```bash
ancestry-tag-converter --original-data discard --custom-tags event MyTree.ged MyTree_ftm.ged
```

**Merge into an existing FTM tree:**
```bash
ancestry-tag-converter --merge FamilyTree.ged MyTree.ged merged.ged
```
The tool automatically detects which file is the Ancestry export and which is the FTM base, so argument order doesn't matter.

**Convert without adding media records or relationship tags:**
```bash
ancestry-tag-converter --no-frel --no-media MyTree.ged MyTree_ftm.ged
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
./ancestry-tag-converter-gui
```

The window presents all the same options as the CLI:

1. **Ancestry file** — browse to your Ancestry GEDCOM export.
2. **Output file** — the output path is auto-suggested based on the input filename (`MyTree_ftm.ged` or `MyTree_merged.ged`). You can change it.
3. **FTM base file** — enabled when "Merge into existing FTM base file" is checked.
4. **Original data** — keep all input data (default) or discard data replaced/removed by conversion.
5. **Custom tags as** — create `FACT` records (default) or GEDCOM `EVEN` records.
6. **Options** — checkboxes for skipping relationship tags and/or media records.
7. **Convert** — starts conversion. If the output file exists, a confirmation dialog appears first.

A progress bar tracks each phase. A log area at the bottom shows matched/unmatched counts and conversion summaries after the run completes.

Open the built-in help with **F1** on Windows/Linux or **Command+?** on macOS.

## Merge behavior

When `--merge` is used, the tool:

1. Loads the entire FTM base file into memory and indexes all individuals by a **match key** (normalized full name + birth year).
2. Converts each individual from the Ancestry file.
3. For each Ancestry individual, finds the matching FTM individual (if any) and adds events that don't already exist in the FTM record.

**What gets merged:** Any event not already present — `FACT`, `OCCU`, `RESI`, `EDUC`, `EVEN`, custom tags, etc. When original data is kept, `_MTTAG` references on matched individuals and their referenced `_MTTAG`/`_MTCAT` definitions are also retained. Definition XRefs are remapped if they collide with the FTM base.

**What is never merged:** `NAME`, `SEX`, family links (`FAMC`/`FAMS`), source citations (`SOUR`), media links (`OBJE`), and structural IDs (`REFN`, `RIN`, `CHAN`).

**Singleton events** (birth, death, baptism, burial, etc.) are skipped if the FTM record already has one, regardless of the date or place.

**Deduplication** compares a canonical signature of each event: `TAG|val:…|type:…|date:…|plac:…`. Two events with the same signature are considered duplicates.

Individuals who appear in Ancestry but have no name+year match in the FTM base are counted as "unmatched" (reported with `--verbose` or in the GUI log).

Running conversion again on its own output, or repeating the same merge, does not duplicate converted fields or retained custom-tag definitions.

## Limitations

- **Ancestry XRef IDs** (`@I102667033170@`) differ from FTM IDs (`@I21@`), so cross-file matching relies on name + birth year rather than ID. Individuals without a birth year or with ambiguous names may not match.
- **Media files** are not transferred — Ancestry GEDCOM exports don't include local file paths, only Ancestry-internal references. The structure is preserved for manual re-linking.
- **Source records** from Ancestry (`SOUR` XRefs) are not merged, as the Ancestry source registry doesn't exist in the FTM file.
- **Family matching** in merge mode only operates on individuals (`INDI`); family (`FAM`) records are not merged.

## Privacy Policy

[PRIVACY_POLICY.md](PRIVACY_POLICY.md)

# AGENTS.md

@/home/adam/.codex/RTK.md

## Project Notes

- This is a Go GEDCOM converter with separate CLI (`main.go`) and Fyne GUI
  (`cmd/ancestry-tag-converter-gui/main.go`) orchestration. Keep their options
  and defaults synchronized.
- Conversion is record-at-a-time, but Ancestry `_MTTAG` references require a
  first pass to load `_MTTAG` and `_MTCAT` definitions before conversion.
- The default is to preserve all original input data and add converted
  equivalents. `--original-data discard` enables destructive cleanup. Explicit
  removal options such as `--no-media` still take priority.
- The GUI label `EVENT` maps to the standard GEDCOM tag `EVEN`; do not emit a
  literal `EVENT` tag.
- GUI help content is embedded from `help/HELP.md`. Keep its option descriptions
  and keyboard shortcuts synchronized with the GUI and README.
- Conversion and merge behavior must remain idempotent. Reprocessing converted
  output or repeating a merge must not duplicate generated fields, relationship
  tags, custom-tag references, or definition records.
- In merge mode, original data retention applies only to custom-tag references
  on matched individuals and their referenced `_MTTAG`/`_MTCAT` definitions.
  Definitions must be collision-safe, remapped consistently, and inserted
  before `TRLR`.
- Run `rtk go test ./...` for converter regressions. Building the GUI may require
  CGO and platform graphics development libraries; the CLI does not.

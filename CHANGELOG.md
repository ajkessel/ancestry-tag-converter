# Changelog

## [0.0.5]

### Added

- Added built-in GUI help, available from the Help menu, **F1** on
  Windows/Linux, or **Command+?** on macOS.
- Added CLI and GUI options to keep or discard original Ancestry data. Original
  data is kept by default.
- Added CLI and GUI options to convert custom tags to `FACT` records by default
  or standard GEDCOM `EVEN` records.
- Added `REFN` as a third custom-tag conversion format, preserving the Ancestry
  tag ID as the reference value and the tag name as `TYPE`.
- Added collision-safe retention of referenced `_MTTAG` and `_MTCAT`
  definitions when merging into an FTM base.
- Added converter regression tests for data retention, custom event output,
  media removal, repeated conversion, and repeated merge behavior.
- Added repository guidance for coding agents in `AGENTS.md`.

### Changed

- Protect against overwriting either of the input files; output must be a different file
- Routed F1 through focused GUI controls so the help shortcut works regardless
  of which form field or button currently has keyboard focus.
- Converted fields are now added alongside their original GEDCOM fields when
  original data is kept.
- Repeated conversions and merges are idempotent and no longer add duplicate
  converted fields, relationship tags, or custom-tag definitions.
- Updated the README and technical reference for the new defaults, interfaces,
  merge behavior, and idempotency guarantees.

## [0.0.3]

- Bug fixes, still getting to MVP

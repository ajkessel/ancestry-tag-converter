# Ancestry Tag Converter Help

This application converts Ancestry GEDCOM exports into a form that is more
compatible with Family Tree Maker and other GEDCOM applications.

## Files

1. Select the **Ancestry file** to convert.
2. Choose the **Output file** to create.
3. To merge converted records into an existing Family Tree Maker export, enable
   **Merge into existing FTM base file** and select the base file.

The application never modifies the selected input or base file.

## Options

**Original data**

- **Keep** preserves the original Ancestry records and fields while adding
  converted equivalents. This is the default.
- **Discard** removes Ancestry-only data that is replaced or removed during
  conversion.

**Custom tags as**

- **FACT** creates GEDCOM `FACT` records. This is the default.
- **EVENT** creates standard GEDCOM `EVEN` records.

**Skip relationship tags** prevents the addition of `_FREL Natural` and
`_MREL Natural` to child records.

**Skip media records** removes top-level media records and inline media
references, even when original data is kept.

## Merge Behavior

Merge mode preserves the FTM base and adds only events that are not already
present. People are matched using normalized name and birth year.

When original data is kept, custom-tag references and the definitions needed by
matched people are also retained. Other original Ancestry-only data is not
copied into the FTM base.

Repeated conversions and repeated merges do not add duplicate converted fields.

## Keyboard Shortcut

- Windows and Linux: **F1**
- macOS: **Command+?**

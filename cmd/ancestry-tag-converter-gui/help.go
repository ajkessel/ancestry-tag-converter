package main

import (
	_ "embed"
	"fmt"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// appIconPNG is the application icon, embedded so the running app shows it in
// the window title bar, taskbar, and dock on every platform — including Linux
// and Windows builds produced by a plain `go build` (which embeds no icon of
// its own). Generated from packaging/icon_full.png.
//
//go:embed appicon.png
var appIconPNG []byte

// appIcon is the embedded icon as a Fyne resource for app.SetIcon.
var appIcon = fyne.NewStaticResource("appicon.png", appIconPNG)

// appName is the human-readable product name shown in the Help and About dialogs.
const appName = "Ancestry Tag Converter"

// copyright is the copyright line shown in the About dialog. Keep in sync with LICENSE.
const copyright = "Copyright © 2026 Adam J. Kessel"

// licenseURL points at the project license for the About dialog reference.
const licenseURL = "https://github.com/ajkessel/ancestry-tag-converter/blob/main/LICENSE"

// version is set at build time via -ldflags "-X main.version=...". Every build
// path (the Makefile and all CI workflows, including the fyne-packaged macOS and
// Windows binaries via GOFLAGS) injects it. We deliberately do NOT fall back to
// fyne.App.Metadata().Version: in -release builds that returns Fyne's hardcoded
// "0.0.1" default rather than the packaged -appVersion, which would silently
// mask a missing injection.
var version string

func appVersion() string {
	if version != "" {
		return version
	}
	return "dev"
}

// helpMarkdown is the Background and Features content from the README, shown in
// the Help dialog.
const helpMarkdown = `## Background

This tool helps bridge the gap between Ancestry and other genealogy platforms.

Ancestry provides a "MyTreeTags" feature that allows users to apply custom tags to their trees. When an Ancestry tree is exported to GEDCOM, these tags are preserved, but not in a GEDCOM-standard compliant way. Users who sync their trees to Family Tree Maker (or import their Ancestry exported GEDCOM into FTM) lose access to these custom tags. A separate problem is that Ancestry provides no mechanism to download media (such as profile photos in your tree and other image/video/PDF content).

FTM, by contrast, allows users to export a synced Ancestry tree to a GEDCOM file with all media preserved. This GEDCOM file can then be opened in other standards-compliant applications with media like images preserved. But this file will not have any custom tags created in Ancestry.

This tool bridges that gap: it strips Ancestry-internal tags, converts Ancestry conventions to FTM conventions, and can selectively merge new data from an Ancestry export into your existing FTM GEDCOM export without duplicating events you already have.

This tool does not alter your existing GEDCOM files. Instead, it creates a new output/merged file.

This tool has no network interaction and does not retain any information.

## Features

- Strips 20+ Ancestry-internal tags (` + "`_APID`, `_OID`, `_CLON`, `_META`, `_WLNK`" + `, …)
- Converts source citation URLs (` + "`DATA/WWW`" + `) to FTM's ` + "`_LINK` + `NOTE`" + ` format
- Adds ` + "`_FREL Natural` / `_MREL Natural`" + ` after each ` + "`CHIL`" + ` record in families
- Converts media dates (` + "`DATE` → `_DATE`" + ` inside ` + "`OBJE`" + ` records)
- Converts graduation school names from ` + "`NOTE`" + ` children to inline ` + "`GRAD`" + ` values
- Converts ` + "`_MTTAG`" + ` DNA/matching tags to human-readable ` + "`FACT`" + ` entries (two-pass lookup)
- Merge mode: preserves all data from an existing FTM file, adding only new events from Ancestry without duplicating anything
- Automatic argument-order detection (swaps Ancestry/FTM files if passed in the wrong order)
- Available as both a command-line tool and a native GUI`

// buildMainMenu builds the application menu. Fyne's native-menu handling places
// a menu labelled "Help" and an item labelled "About" in each platform's
// conventional location (the Help menu, and the app menu on macOS).
func buildMainMenu(w fyne.Window) *fyne.MainMenu {
	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem(appName+" Help", func() { showHelp(w) }),
		fyne.NewMenuItem("About", func() { showAbout(w) }),
	)
	return fyne.NewMainMenu(helpMenu)
}

func showHelp(w fyne.Window) {
	content := widget.NewRichTextFromMarkdown(helpMarkdown)
	content.Wrapping = fyne.TextWrapWord

	scroll := container.NewScroll(content)
	scroll.SetMinSize(fyne.NewSize(560, 460))

	d := dialog.NewCustom(appName+" Help", "Close", scroll, w)
	d.Show()
}

func showAbout(w fyne.Window) {
	licenseLink := widget.NewHyperlink("BSD 2-Clause License", parseURL(licenseURL))

	title := widget.NewLabelWithStyle(appName, fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true})
	ver := widget.NewLabelWithStyle("Version "+appVersion(), fyne.TextAlignCenter,
		fyne.TextStyle{})
	copy := widget.NewLabelWithStyle(copyright, fyne.TextAlignCenter,
		fyne.TextStyle{})
	licenseRow := container.NewCenter(container.NewHBox(
		widget.NewLabel("Licensed under the"), licenseLink,
	))

	content := container.NewVBox(title, ver, copy, licenseRow)

	d := dialog.NewCustom("About "+appName, "Close", content, w)
	d.Show()
}

func parseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		// A malformed constant is a programmer error; surface it loudly in dev
		// rather than silently dropping the link.
		panic(fmt.Sprintf("invalid license URL %q: %v", raw, err))
	}
	return u
}

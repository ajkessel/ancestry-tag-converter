package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/ncruces/zenity"

	"github.com/ajkessel/ancestry-tag-converter/converter"
	"github.com/ajkessel/ancestry-tag-converter/gedcom"
	apphelp "github.com/ajkessel/ancestry-tag-converter/help"
	"github.com/ajkessel/ancestry-tag-converter/internal/pathcheck"
)

func main() {
	a := app.NewWithID("com.ajkessel.ancestry-tag-converter")
	w := a.NewWindow("Ancestry → FTM Converter")
	w.Resize(fyne.NewSize(640, 600))

	showHelp := func() {
		helpWindow := a.NewWindow("Ancestry Tag Converter Help")
		helpText := widget.NewRichTextFromMarkdown(apphelp.Markdown)
		helpText.Wrapping = fyne.TextWrapWord
		helpWindow.SetContent(container.NewPadded(container.NewScroll(helpText)))
		helpWindow.Resize(fyne.NewSize(640, 600))
		helpWindow.Show()
	}
	helpShortcut := helpKeyboardShortcut(runtime.GOOS)
	helpItem := fyne.NewMenuItem("Help", showHelp)
	helpItem.Shortcut = helpShortcut
	w.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("Help", helpItem)))
	w.Canvas().AddShortcut(helpShortcut, func(fyne.Shortcut) {
		showHelp()
	})
	if runtime.GOOS != "darwin" {
		if desktopCanvas, ok := w.Canvas().(desktop.Canvas); ok {
			desktopCanvas.SetOnKeyDown(func(event *fyne.KeyEvent) {
				if event.Name == fyne.KeyF1 {
					showHelp()
				}
			})
		}
	}

	// ── File entries ──────────────────────────────────────────────────────────
	inputEntry := newHelpEntry(showHelp, false)
	inputEntry.SetPlaceHolder("Ancestry GEDCOM export…")
	outputEntry := newHelpEntry(showHelp, false)
	outputEntry.SetPlaceHolder("Output file…")
	mergeEntry := newHelpEntry(showHelp, false)
	mergeEntry.SetPlaceHolder("FTM base GEDCOM file…")
	mergeEntry.Disable()

	// ── Options ───────────────────────────────────────────────────────────────
	mergeCheck := newHelpCheck(showHelp, "Merge into existing FTM base file", nil)
	noFRelCheck := newHelpCheck(showHelp, "Skip _FREL/_MREL Natural (relationship tags)", nil)
	noMediaCheck := newHelpCheck(showHelp, "Skip media records (OBJE)", nil)
	originalDataSelect := newHelpSelect(showHelp, []string{"Keep", "Discard"}, nil)
	originalDataSelect.SetSelected("Keep")
	customTagSelect := newHelpSelect(showHelp, []string{"FACT", "EVENT", "REFN"}, nil)
	customTagSelect.SetSelected("FACT")

	// ── Progress ──────────────────────────────────────────────────────────────
	progressBar := widget.NewProgressBar()
	phaseLabel := widget.NewLabel("Ready.")
	logBox := newHelpEntry(showHelp, true)
	logBox.Wrapping = fyne.TextWrapWord
	logBox.SetMinRowsVisible(6)

	// ── Browse helpers (native OS file dialogs via zenity) ───────────────────
	gedFilter := zenity.FileFilter{Name: "GEDCOM Files", Patterns: []string{"*.ged", "*.GED"}}

	openGED := func(callback func(string)) {
		go func() {
			p, err := zenity.SelectFile(zenity.Title("Select GEDCOM file"), gedFilter)
			if err == nil && p != "" {
				callback(p)
			}
		}()
	}
	saveGED := func(callback func(string)) {
		go func() {
			p, err := zenity.SelectFileSave(
				zenity.Title("Save output as"),
				zenity.Filename("output.ged"),
				zenity.ConfirmOverwrite(),
				gedFilter,
			)
			if err == nil && p != "" {
				if !strings.HasSuffix(strings.ToLower(p), ".ged") {
					p += ".ged"
				}
				callback(p)
			}
		}()
	}

	mergeBrowseBtn := newHelpButton(showHelp, "Browse…", func() { openGED(mergeEntry.SetText) })
	mergeBrowseBtn.Disable()

	// Auto-suggest output filename whenever the input path or merge toggle changes.
	suggestOutput := func() {
		if inputEntry.Text == "" {
			return
		}
		ext := filepath.Ext(inputEntry.Text)
		base := strings.TrimSuffix(inputEntry.Text, ext)
		suffix := "_ftm"
		if mergeCheck.Checked {
			suffix = "_merged"
		}
		suggested := base + suffix + ext
		cur := outputEntry.Text
		if cur == "" || strings.HasSuffix(cur, "_ftm.ged") || strings.HasSuffix(cur, "_merged.ged") {
			outputEntry.SetText(suggested)
		}
	}
	inputEntry.OnChanged = func(_ string) { suggestOutput() }

	mergeCheck.OnChanged = func(checked bool) {
		if checked {
			mergeEntry.Enable()
			mergeBrowseBtn.Enable()
		} else {
			mergeEntry.Disable()
			mergeBrowseBtn.Disable()
		}
		suggestOutput()
	}

	// ── Layout helpers ────────────────────────────────────────────────────────
	fileRow := func(entry, btn fyne.CanvasObject) *fyne.Container {
		return container.NewBorder(nil, nil, nil, btn, entry)
	}

	// ── Convert action ────────────────────────────────────────────────────────
	var convertBtn *helpButton

	doConvert := func() {
		input := inputEntry.Text
		output := outputEntry.Text
		mergeBase := mergeEntry.Text
		doMerge := mergeCheck.Checked
		noFRel := noFRelCheck.Checked
		noMedia := noMediaCheck.Checked
		originalData := converter.OriginalDataMode(strings.ToLower(originalDataSelect.Selected))
		customTags := converter.CustomTagRecord(strings.ToLower(customTagSelect.Selected))

		convertBtn.Disable()
		logBox.SetText("")
		go func() {
			defer convertBtn.Enable()
			runConversion(input, output, mergeBase, doMerge, noFRel, noMedia, originalData, customTags,
				progressBar, phaseLabel, logBox, w)
		}()
	}

	convertBtn = newHelpButton(showHelp, "Convert", func() {
		input := inputEntry.Text
		output := outputEntry.Text
		doMerge := mergeCheck.Checked
		mergeBase := mergeEntry.Text

		switch {
		case input == "":
			dialog.ShowError(fmt.Errorf("no Ancestry input file selected"), w)
			return
		case output == "":
			dialog.ShowError(fmt.Errorf("no output file specified"), w)
			return
		case doMerge && mergeBase == "":
			dialog.ShowError(fmt.Errorf("merge enabled but no FTM base file selected"), w)
			return
		}

		inputPaths := []string{input}
		if doMerge {
			inputPaths = append(inputPaths, mergeBase)
		}
		if err := pathcheck.EnsureOutputDistinct(output, inputPaths...); err != nil {
			dialog.ShowError(err, w)
			return
		}

		if _, err := os.Stat(output); err == nil {
			dialog.ShowConfirm("File exists",
				fmt.Sprintf("%s already exists.\nOverwrite?", filepath.Base(output)),
				func(ok bool) {
					if ok {
						doConvert()
					}
				}, w)
			return
		}
		doConvert()
	})

	// ── Form ──────────────────────────────────────────────────────────────────
	form := widget.NewForm(
		widget.NewFormItem("Ancestry file:", fileRow(inputEntry, newHelpButton(showHelp, "Browse…", func() {
			openGED(func(p string) { inputEntry.SetText(p) })
		}))),
		widget.NewFormItem("Output file:", fileRow(outputEntry, newHelpButton(showHelp, "Browse…", func() {
			saveGED(func(p string) { outputEntry.SetText(p) })
		}))),
		widget.NewFormItem("FTM base file:", fileRow(mergeEntry, mergeBrowseBtn)),
	)

	content := container.NewVBox(
		form,
		mergeCheck,
		widget.NewSeparator(),
		widget.NewForm(
			widget.NewFormItem("Original data:", originalDataSelect),
			widget.NewFormItem("Custom tags as:", customTagSelect),
		),
		noFRelCheck,
		noMediaCheck,
		widget.NewSeparator(),
		convertBtn,
		progressBar,
		phaseLabel,
		widget.NewSeparator(),
		container.NewScroll(logBox),
	)

	w.SetContent(container.NewPadded(content))
	w.ShowAndRun()
}

func helpKeyboardShortcut(goos string) *desktop.CustomShortcut {
	if goos == "darwin" {
		return &desktop.CustomShortcut{
			KeyName:  fyne.KeySlash,
			Modifier: fyne.KeyModifierSuper | fyne.KeyModifierShift,
		}
	}
	return &desktop.CustomShortcut{KeyName: fyne.KeyF1}
}

func isHelpKey(event *fyne.KeyEvent) bool {
	return event != nil && event.Name == fyne.KeyF1
}

type helpEntry struct {
	widget.Entry
	showHelp func()
}

func newHelpEntry(showHelp func(), multiline bool) *helpEntry {
	entry := &helpEntry{showHelp: showHelp}
	entry.MultiLine = multiline
	entry.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)
	entry.ExtendBaseWidget(entry)
	return entry
}

func (e *helpEntry) KeyDown(event *fyne.KeyEvent) {
	if isHelpKey(event) {
		e.showHelp()
		return
	}
	e.Entry.KeyDown(event)
}

type helpCheck struct {
	widget.Check
	showHelp func()
}

func newHelpCheck(showHelp func(), label string, changed func(bool)) *helpCheck {
	check := &helpCheck{showHelp: showHelp}
	check.Text = label
	check.OnChanged = changed
	check.ExtendBaseWidget(check)
	return check
}

func (c *helpCheck) TypedKey(event *fyne.KeyEvent) {
	if isHelpKey(event) {
		c.showHelp()
		return
	}
	c.Check.TypedKey(event)
}

type helpSelect struct {
	widget.Select
	showHelp func()
}

func newHelpSelect(showHelp func(), options []string, changed func(string)) *helpSelect {
	selectWidget := &helpSelect{showHelp: showHelp}
	selectWidget.Options = options
	selectWidget.PlaceHolder = "(Select one)"
	selectWidget.OnChanged = changed
	selectWidget.ExtendBaseWidget(selectWidget)
	return selectWidget
}

func (s *helpSelect) TypedKey(event *fyne.KeyEvent) {
	if isHelpKey(event) {
		s.showHelp()
		return
	}
	s.Select.TypedKey(event)
}

type helpButton struct {
	widget.Button
	showHelp func()
}

func newHelpButton(showHelp func(), label string, tapped func()) *helpButton {
	button := &helpButton{showHelp: showHelp}
	button.Text = label
	button.OnTapped = tapped
	button.ExtendBaseWidget(button)
	return button
}

func (b *helpButton) TypedKey(event *fyne.KeyEvent) {
	if isHelpKey(event) {
		b.showHelp()
		return
	}
	b.Button.TypedKey(event)
}

// ── Conversion logic ──────────────────────────────────────────────────────────

type phaseBar struct {
	bar   *widget.ProgressBar
	label *widget.Label
	base  float64
	span  float64
	total int64
	done  int64
}

func (p *phaseBar) begin(label string, base, span float64, total int64) {
	p.base, p.span, p.total, p.done = base, span, total, 0
	p.label.SetText(label)
}

func (p *phaseBar) add(n int64) {
	if p.total <= 0 {
		return
	}
	p.done += n
	frac := float64(p.done) / float64(p.total)
	if frac > 1 {
		frac = 1
	}
	p.bar.SetValue(p.base + frac*p.span)
}

func (p *phaseBar) finish() {
	p.bar.SetValue(p.base + p.span)
}

// phaseReader wraps an io.Reader, advancing the phase bar as bytes are read.
type phaseReader struct {
	r io.Reader
	p *phaseBar
}

func (r *phaseReader) Read(buf []byte) (n int, err error) {
	n, err = r.r.Read(buf)
	if n > 0 {
		r.p.add(int64(n))
	}
	return
}

func appendLog(box *helpEntry, msg string) {
	cur := box.Text
	if cur != "" {
		cur += "\n"
	}
	box.SetText(cur + msg)
}

func runConversion(
	inputPath, outputPath, mergeBasePath string,
	doMerge, noFRel, noMedia bool,
	originalData converter.OriginalDataMode,
	customTags converter.CustomTagRecord,
	bar *widget.ProgressBar,
	phaseLabel *widget.Label,
	logBox *helpEntry,
	win fyne.Window,
) {
	bar.SetValue(0)

	inputPaths := []string{inputPath}
	if doMerge {
		inputPaths = append(inputPaths, mergeBasePath)
	}
	if err := pathcheck.EnsureOutputDistinct(outputPath, inputPaths...); err != nil {
		dialog.ShowError(err, win)
		return
	}

	// Auto-detect argument order: if the "ancestry" file looks like FTM and vice versa, swap.
	if doMerge {
		if isAncestry(mergeBasePath) && !isAncestry(inputPath) {
			appendLog(logBox, fmt.Sprintf("note: swapping — %s is the Ancestry file, %s is the FTM base",
				filepath.Base(mergeBasePath), filepath.Base(inputPath)))
			inputPath, mergeBasePath = mergeBasePath, inputPath
		}
	}

	pb := &phaseBar{bar: bar, label: phaseLabel}

	inputSize := gedSize(inputPath)

	// ── Phase 1: scan MT tags (0% → 20%) ─────────────────────────────────────
	pb.begin("Scanning tags…", 0.0, 0.20, inputSize)
	scanFile, err := os.Open(inputPath)
	if err != nil {
		dialog.ShowError(err, win)
		return
	}
	mttagMap, mtcatMap, err := converter.ScanMTTagsFromReader(&phaseReader{r: scanFile, p: pb})
	scanFile.Close()
	if err != nil {
		dialog.ShowError(err, win)
		return
	}
	pb.finish()

	opts := converter.Options{
		NoFRel:          noFRel,
		NoMedia:         noMedia,
		OriginalData:    originalData,
		CustomTagRecord: customTags,
		MTTagMap:        mttagMap,
		MTCatMap:        mtcatMap,
	}
	stats := converter.NewStats()
	start := time.Now()

	// Create output file early so we can catch permission errors before doing work.
	out, err := os.Create(outputPath)
	if err != nil {
		dialog.ShowError(err, win)
		return
	}
	defer out.Close()
	bw := bufio.NewWriterSize(out, 4*1024*1024)

	if doMerge {
		// ── Phase 2: load FTM base (20% → 50%) ───────────────────────────────
		pb.begin("Loading base…", 0.20, 0.30, gedSize(mergeBasePath))
		baseFile, err := os.Open(mergeBasePath)
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		base, err := converter.LoadAndIndexFromReader(&phaseReader{r: baseFile, p: pb})
		baseFile.Close()
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		pb.finish()
		customTagPlan := converter.PrepareCustomTagMerge(base, opts)

		// ── Phase 3: convert Ancestry INDIs (50% → 85%) ──────────────────────
		pb.begin("Converting…", 0.50, 0.35, inputSize)
		in, err := os.Open(inputPath)
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		matched, unmatched := 0, 0
		parser := gedcom.NewParser(&phaseReader{r: in, p: pb})
		for {
			rec := parser.Next()
			if rec == nil {
				break
			}
			conv := converter.Convert(rec, stats, opts)
			if conv == nil || conv.Tag != "INDI" {
				continue
			}
			key := converter.IndividualKey(conv)
			if baseIndi, ok := base.IndiByKey[key]; ok {
				customTagPlan.RewriteAndMarkINDI(conv)
				converter.MergeINDIWithOptions(baseIndi, conv, stats, opts)
				matched++
			} else if baseIndi, _ := base.FuzzyMatchINDI(conv); baseIndi != nil {
				customTagPlan.RewriteAndMarkINDI(conv)
				converter.MergeINDIWithOptions(baseIndi, conv, stats, opts)
				matched++
			} else {
				unmatched++
			}
		}
		in.Close()
		pb.finish()
		customTagPlan.AppendDefinitions()

		// ── Phase 4: write merged output (85% → 100%) ────────────────────────
		total := int64(len(base.Records))
		pb.begin("Writing…", 0.85, 0.15, total)
		for _, rec := range base.Records {
			if err := gedcom.WriteRecord(bw, rec); err != nil {
				dialog.ShowError(err, win)
				return
			}
			pb.add(1)
		}
		pb.finish()

		stats.Converted["merge:matched"] = matched
		stats.Converted["merge:unmatched"] = unmatched
	} else {
		// ── Phase 2: convert and write (20% → 100%) ──────────────────────────
		pb.begin("Converting…", 0.20, 0.80, inputSize)
		in, err := os.Open(inputPath)
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		parser := gedcom.NewParser(&phaseReader{r: in, p: pb})
		for {
			rec := parser.Next()
			if rec == nil {
				break
			}
			conv := converter.Convert(rec, stats, opts)
			if conv == nil {
				continue
			}
			if err := gedcom.WriteRecord(bw, conv); err != nil {
				dialog.ShowError(err, win)
				return
			}
		}
		in.Close()
		fmt.Fprintln(bw, "0 TRLR")
		pb.finish()
	}

	if err := bw.Flush(); err != nil {
		dialog.ShowError(err, win)
		return
	}

	elapsed := time.Since(start).Round(time.Millisecond)
	phaseLabel.SetText(fmt.Sprintf("Done in %s.", elapsed))
	bar.SetValue(1.0)

	if doMerge {
		matched := stats.Converted["merge:matched"]
		unmatched := stats.Converted["merge:unmatched"]
		skipped := stats.Converted["merge:skipped"]
		appendLog(logBox, fmt.Sprintf("Merge complete in %s: %d matched, %d unmatched, %d duplicate events skipped",
			elapsed, matched, unmatched, skipped))
	} else {
		total := 0
		for _, n := range stats.Records {
			total += n
		}
		appendLog(logBox, fmt.Sprintf("Conversion complete in %s: %d records processed", elapsed, total))
	}
	appendLog(logBox, fmt.Sprintf("Records: INDI=%d  FAM=%d  OBJE=%d  SOUR=%d",
		stats.Records["INDI"], stats.Records["FAM"],
		stats.Records["OBJE"], stats.Records["SOUR"]))
	if dropped := totalDropped(stats); dropped > 0 {
		appendLog(logBox, fmt.Sprintf("Dropped %d Ancestry-internal tags", dropped))
	}
	if conv := stats.Converted["DATA/WWW→_LINK"]; conv > 0 {
		appendLog(logBox, fmt.Sprintf("Converted %d source URLs (DATA/WWW → _LINK)", conv))
	}
	if conv := stats.Converted["CHIL→_FREL/_MREL"]; conv > 0 {
		appendLog(logBox, fmt.Sprintf("Added _FREL/_MREL to %d CHIL records", conv))
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func gedSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func isAncestry(path string) bool {
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

func totalDropped(s *converter.Stats) int {
	n := 0
	for _, v := range s.Dropped {
		n += v
	}
	return n
}

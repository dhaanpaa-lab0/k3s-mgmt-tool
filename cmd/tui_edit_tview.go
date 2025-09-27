package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"nexus-sites.net/k3s-mgmt-tool/pkg/buildfile"
)

// editCmd opens a full-screen interactive editor for Buildfile.yaml using tview
var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Interactively edit Buildfile.yaml in a full-screen TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := buildfile.LoadFromFile()
		if err != nil {
			return fmt.Errorf("failed to load Buildfile.yaml: %w", err)
		}
		f.EnsureInit()
		return runTviewEditor(f)
	},
}

// Sections
const (
	secRepos = iota
	secCharts
	secManifests
	secScripts
	secGit
)

var sectionNames = []string{
	"1 Helm Repos",
	"2 Helm Charts",
	"3 Manifests",
	"4 Scripts",
	"5 Git Repos",
}

func runTviewEditor(f *buildfile.SetupFile) error {
	app := tview.NewApplication()

 status := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[yellow]F2 Save  Ctrl+S Save  Ctrl+Q Quit  Enter edit  Del delete  Esc close modal  F5 Add  F6 Edit  F7 Delete  F8 Toggle Left/Right")
	status.SetBorder(true).SetTitle("Status")

	// Left navigation list
	nav := tview.NewList().ShowSecondaryText(false)
	nav.SetBorder(true).SetTitle("Sections")

	contentPages := tview.NewPages()

	// We'll use a single root pages container and add modals directly to it.
	root := tview.NewPages()

	// Build content pages
	contentPages.AddPage(pageNameFor(secRepos), buildMapEditor(app, root, status, f, secRepos), true, true)
	contentPages.AddPage(pageNameFor(secCharts), buildMapEditor(app, root, status, f, secCharts), true, false)
	contentPages.AddPage(pageNameFor(secManifests), buildListEditor(app, root, status, f, secManifests), true, false)
	contentPages.AddPage(pageNameFor(secScripts), buildListEditor(app, root, status, f, secScripts), true, false)
	contentPages.AddPage(pageNameFor(secGit), buildMapEditor(app, root, status, f, secGit), true, false)

	for i, name := range sectionNames {
		i := i
		nav.AddItem(name, "", rune('1'+i), func() {
			contentPages.SwitchToPage(pageNameFor(i))
		})
	}

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nav, 24, 0, true).
			AddItem(contentPages, 0, 1, false), 0, 1, true).
		AddItem(status, 1, 0, false)

	root.AddPage("main", layout, true, true)

	// Global keybindings
	app.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyCtrlS, tcell.KeyF2:
			if err := f.SaveToFile(); err != nil {
				status.SetText("[red]Save failed: " + err.Error())
			} else if _, err := buildfile.LoadFromFile(); err != nil {
				status.SetText("[yellow]Saved, but validation failed: " + err.Error())
			} else {
				status.SetText("[green]Saved successfully.")
			}
			return nil
		case tcell.KeyCtrlQ:
			app.Stop()
			return nil
		case tcell.KeyF8:
			// Toggle focus between left (nav) and right (content)
			if app.GetFocus() == nav {
				// Focus the current right content page's root primitive
				if _, item := contentPages.GetFrontPage(); item != nil {
					app.SetFocus(item)
				} else {
					// Fallback: focus content container
					app.SetFocus(contentPages)
				}
			} else {
				app.SetFocus(nav)
			}
			return nil
		}
		return ev
	})

	// Keep nav selection synced with pages
	nav.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		contentPages.SwitchToPage(pageNameFor(index))
	})

	if err := app.SetRoot(root, true).EnableMouse(true).Run(); err != nil {
		return fmt.Errorf("tui failed: %w", err)
	}
	return nil
}

func pageNameFor(i int) string { return fmt.Sprintf("page-%d", i) }

// -------- Helpers and editors (tview) --------

func getMapRef(f *buildfile.SetupFile, section int) map[string]string {
	switch section {
	case secRepos:
		return f.HelmRepos
	case secCharts:
		return f.HelmCharts
	case secGit:
		return f.GitRepos
	}
	return nil
}

func mapTitle(section int) string {
	switch section {
	case secRepos:
		return "Helm Repos (name -> URL)"
	case secCharts:
		return "Helm Charts (release -> chart)"
	case secGit:
		return "Git Repos (name -> URL)"
	}
	return "Map"
}

func refreshMapTable(tbl *tview.Table, f *buildfile.SetupFile, section int) {
	tbl.Clear()
	// Header
	tbl.SetCell(0, 0, tview.NewTableCell("Key").SetSelectable(false).SetAttributes(tcell.AttrBold))
	tbl.SetCell(0, 1, tview.NewTableCell("Value").SetSelectable(false).SetAttributes(tcell.AttrBold))
	m := getMapRef(f, section)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		row := i + 1
		tbl.SetCell(row, 0, tview.NewTableCell(k))
		tbl.SetCell(row, 1, tview.NewTableCell(m[k]))
	}
}

func buildMapEditor(app *tview.Application, host *tview.Pages, status *tview.TextView, f *buildfile.SetupFile, section int) tview.Primitive {
	table := tview.NewTable().SetSelectable(true, false)
	table.SetBorder(true).SetTitle(mapTitle(section))
	refreshMapTable(table, f, section)

	showForm := func(origKey, origVal string) {
		form := tview.NewForm().AddInputField("Key", origKey, 0, nil, nil).AddInputField("Value", origVal, 0, nil, nil)
		// Enforce 255-character max for both fields while typing
		if0 := form.GetFormItem(0).(*tview.InputField)
		if1 := form.GetFormItem(1).(*tview.InputField)
		limit := func(text string, last rune) bool { return len([]rune(text)) <= 255 }
		if0.SetAcceptanceFunc(limit)
		if1.SetAcceptanceFunc(limit)
		form.AddButton("Save", func() {
			newKey := strings.TrimSpace(form.GetFormItem(0).(*tview.InputField).GetText())
			newVal := strings.TrimSpace(form.GetFormItem(1).(*tview.InputField).GetText())
			if newKey != "" && newVal != "" {
				if origKey != "" && origKey != newKey {
					// rename: remove old
					switch section {
					case secRepos:
						_ = f.RemoveHelmRepo(origKey)
					case secCharts:
						_ = f.RemoveHelmChart(origKey)
					case secGit:
						_ = f.RemoveGitRepo(origKey)
					}
				}
				switch section {
				case secRepos:
					f.AddHelmRepo(newKey, newVal)
				case secCharts:
					f.AddHelmChart(newKey, newVal)
				case secGit:
					f.AddGitRepo(newKey, newVal)
				}
				status.SetText("[green]Saved entry")
			}
				host.RemovePage("modal")
				refreshMapTable(table, f, section)
		})
		form.AddButton("Cancel", func() { host.RemovePage("modal") })
		form.SetButtonsAlign(tview.AlignRight)
		form.SetBorder(true).SetTitle("Edit Entry")
		form.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
			if ev.Key() == tcell.KeyEsc { host.RemovePage("modal"); return nil }
			return ev
		})

		modal := centerModal(form, 70, 10)
		host.AddPage("modal", modal, true, true)
	}

	// Buttons
	btnBar := tview.NewFlex().SetDirection(tview.FlexColumn)
	btnAdd := tview.NewButton("Add").SetSelectedFunc(func() { showForm("", "") })
	onEdit := func() {
		row, _ := table.GetSelection()
		if row <= 0 { return }
		key := table.GetCell(row, 0).Text
		val := table.GetCell(row, 1).Text
		showForm(key, val)
	}
	btnEdit := tview.NewButton("Edit").SetSelectedFunc(onEdit)
	onDel := func() {
		row, _ := table.GetSelection()
		if row <= 0 { return }
		key := table.GetCell(row, 0).Text
		var removed bool
		switch section {
		case secRepos:
			removed = f.RemoveHelmRepo(key)
		case secCharts:
			removed = f.RemoveHelmChart(key)
		case secGit:
			removed = f.RemoveGitRepo(key)
		}
		if removed {
			status.SetText("[yellow]Removed " + key)
			refreshMapTable(table, f, section)
		}
	}
	btnDel := tview.NewButton("Delete").SetSelectedFunc(onDel)
	for _, b := range []*tview.Button{btnAdd, btnEdit, btnDel} {
		btnBar.AddItem(b, 12, 0, false)
	}
	btnBar.AddItem(tview.NewBox(), 0, 1, false)

	// Table keys
	table.SetSelectedFunc(func(row, column int) { onEdit() })
	table.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyDelete, tcell.KeyDEL:
			onDel(); return nil
		case tcell.KeyEnter:
			onEdit(); return nil
		case tcell.KeyF5:
			showForm("", ""); return nil
		case tcell.KeyF6:
			onEdit(); return nil
		case tcell.KeyF7:
			onDel(); return nil
		}
		return ev
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(btnBar, 1, 0, false)
	return content
}

func getListRef(f *buildfile.SetupFile, section int) *[]string {
	switch section {
	case secManifests:
		return &f.Manifests
	case secScripts:
		return &f.StartupScripts
	}
	return nil
}

func listTitle(section int) string {
	if section == secManifests { return "K8s Manifests" }
	return "Startup Scripts"
}

func refreshListTable(tbl *tview.Table, f *buildfile.SetupFile, section int) {
	tbl.Clear()
	tbl.SetCell(0, 0, tview.NewTableCell("Item").SetSelectable(false).SetAttributes(tcell.AttrBold))
	arrp := getListRef(f, section)
	arr := append([]string{}, (*arrp)...)
	sort.Strings(arr)
	for i, v := range arr {
		tbl.SetCell(i+1, 0, tview.NewTableCell(v))
	}
}

func buildListEditor(app *tview.Application, host *tview.Pages, status *tview.TextView, f *buildfile.SetupFile, section int) tview.Primitive {
	table := tview.NewTable().SetSelectable(true, false)
	table.SetBorder(true).SetTitle(listTitle(section))
	refreshListTable(table, f, section)

	showForm := func(orig string) {
		form := tview.NewForm().AddInputField("Item", orig, 0, nil, nil)
		// Enforce 255-character max for list item while typing
		if0 := form.GetFormItem(0).(*tview.InputField)
		if0.SetAcceptanceFunc(func(text string, last rune) bool { return len([]rune(text)) <= 255 })
		form.AddButton("Save", func() {
			newV := strings.TrimSpace(form.GetFormItem(0).(*tview.InputField).GetText())
			if newV != "" {
				if orig != "" {
					if section == secManifests {
						_ = f.RemoveManifest(orig)
						f.AddManifest(newV)
					} else {
						_ = f.RemoveStartupScript(orig)
						f.AddStartupScript(newV)
					}
				} else {
					if section == secManifests { f.AddManifest(newV) } else { f.AddStartupScript(newV) }
				}
				status.SetText("[green]Saved item")
			}
			host.RemovePage("modal")
			refreshListTable(table, f, section)
		})
		form.AddButton("Cancel", func() { host.RemovePage("modal") })
		form.SetButtonsAlign(tview.AlignRight)
		form.SetBorder(true).SetTitle("Edit Item")
		form.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
			if ev.Key() == tcell.KeyEsc { host.RemovePage("modal"); return nil }
			return ev
		})

		modal := centerModal(form, 70, 8)
		host.AddPage("modal", modal, true, true)
	}

	btnBar := tview.NewFlex().SetDirection(tview.FlexColumn)
	btnAdd := tview.NewButton("Add").SetSelectedFunc(func() { showForm("") })
	onEdit := func() {
		row, _ := table.GetSelection(); if row <= 0 { return }
		showForm(table.GetCell(row, 0).Text)
	}
	btnEdit := tview.NewButton("Edit").SetSelectedFunc(onEdit)
	onDel := func() {
		row, _ := table.GetSelection(); if row <= 0 { return }
		v := table.GetCell(row, 0).Text
		var removed bool
		if section == secManifests { removed = f.RemoveManifest(v) } else { removed = f.RemoveStartupScript(v) }
		if removed { status.SetText("[yellow]Removed " + v); refreshListTable(table, f, section) }
	}
	btnDel := tview.NewButton("Delete").SetSelectedFunc(onDel)
	for _, b := range []*tview.Button{btnAdd, btnEdit, btnDel} {
		btnBar.AddItem(b, 12, 0, false)
	}
	btnBar.AddItem(tview.NewBox(), 0, 1, false)

	table.SetSelectedFunc(func(row, column int) { onEdit() })
	table.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyDelete, tcell.KeyDEL:
			onDel(); return nil
		case tcell.KeyEnter:
			onEdit(); return nil
		case tcell.KeyF5:
			showForm(""); return nil
		case tcell.KeyF6:
			onEdit(); return nil
		case tcell.KeyF7:
			onDel(); return nil
		}
		return ev
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(btnBar, 1, 0, false)
	return content
}

func centerModal(p tview.Primitive, width, height int) tview.Primitive {
	frame := tview.NewFrame(p).SetBorders(0, 0, 0, 0, 1, 1)
	frame.SetBorder(true)
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(frame, width, 0, true).
			AddItem(nil, 0, 1, false), height, 0, true).
		AddItem(nil, 0, 1, false)
	return flex
}

func init() { rootCmd.AddCommand(editCmd) }

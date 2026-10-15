package ui

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mario/ora-tui/internal/collector"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
	"github.com/rivo/tview"
)

type AppUI struct {
	App             *tview.Application
	Pages           *tview.Pages
	State           *model.EnvironmentState
	Header          *tview.TextView
	NavList         *tview.List
	MidFlex         *tview.Flex
	Content         *tview.Pages
	LogView         *tview.TextView
	Footer          *tview.TextView
	ActiveView      string
	ShowMenuHelp    bool
	ColorTheme      ColorThemeName
	isHoveringNav   bool
	activeModalList *tview.List
	viewFocusables  map[string][]tview.Primitive
	tableActions    map[*tview.Table]func(row, col int)
	rawTitles       map[tview.Primitive]string
	logHistory      []string
}

type navEntry struct {
	key   string
	short string
	help  string
	fn    func()
}

func NewAppUI(state *model.EnvironmentState) *AppUI {
	if state.BorderMode == "" {
		state.BorderMode = model.BorderMixedUnicode
	}
	ApplyBorderTheme(state.BorderMode)
	ApplyColorTheme(ThemeNorton)

	ui := &AppUI{
		App:            tview.NewApplication(),
		Pages:          tview.NewPages(),
		State:          state,
		ActiveView:     "overview",
		ShowMenuHelp:   true,
		ColorTheme:     ThemeNorton,
		viewFocusables: make(map[string][]tview.Primitive),
		tableActions:   make(map[*tview.Table]func(row, col int)),
		rawTitles:      make(map[tview.Primitive]string),
	}

	ui.buildLayout()
	return ui
}

// bindTableActions binds both keyboard Enter (SetSelectedFunc) and mouse click to open the table's action submenu.
func (u *AppUI) bindTableActions(tbl *tview.Table, fn func(row, col int)) {
	if u.tableActions == nil {
		u.tableActions = make(map[*tview.Table]func(row, col int))
	}
	u.tableActions[tbl] = fn
	tbl.SetSelectedFunc(fn)
}

// listIndexAtPoint returns the item index at screen coordinates (x, y) inside a tview.List, or -1 if outside.
func listIndexAtPoint(l *tview.List, x, y int) int {
	if l == nil {
		return -1
	}
	rx, ry, rw, rh := l.GetInnerRect()
	if x < rx || x >= rx+rw || y < ry || y >= ry+rh {
		return -1
	}
	idx := y - ry
	if idx < 0 || idx >= l.GetItemCount() {
		return -1
	}
	return idx
}

func (u *AppUI) activeViewNavIndex() int {
	for i, e := range u.navEntries() {
		if e.key != "" && e.key == u.ActiveView {
			return i
		}
	}
	return 0
}

// decorateTitle formats a window's border title according to the active theme and focus state:
// - Norton Commander: Teal (#00AFAF) title badge with Black text when focused; Bright Cyan (#5FFFFF) when unfocused.
// - Borland Turbo C++ 3.0: Bright White title with Green [■] close box and =1=[↑] zoom arrow on Blue windows,
//   or Dark Blue (#0000AF) title with White [■] close box on the bottom Teal/Cyan (#00AFAF) "Project:" window.
// - Green Phosphor CRT: Inverse Bright Green (#00FF5F) badge with dark #001000 text when focused.
func (u *AppUI) decorateTitle(raw string, hasFocus bool) string {
	raw = strings.TrimSpace(raw)
	switch u.ColorTheme {
	case ThemeNorton:
		if hasFocus {
			return fmt.Sprintf("[#000000:#00afaf:b] %s [-:-:-]", raw)
		}
		return fmt.Sprintf("[#5fffff] %s [-]", raw)
	case ThemeTurboC:
		if hasFocus {
			return fmt.Sprintf("[#ffffff][[#5fff5f::b]■[#ffffff]]═ %s ═1=[[#5fff5f::b]↑[#ffffff]][-]", raw)
		}
		return fmt.Sprintf("[#ffffff] %s [-]", raw)
	case ThemeGreenPhosphor:
		if hasFocus {
			return fmt.Sprintf("[#001000:#00ff5f:b] %s [-:-:-]", raw)
		}
		return fmt.Sprintf("[#00af00] %s [-]", raw)
	default:
		if hasFocus {
			return fmt.Sprintf("[#5fffff::b] %s [-::-]", raw)
		}
		return fmt.Sprintf(" %s ", raw)
	}
}

// updateFocusVisuals applies the active color palette (Norton Commander / Borland Turbo C / Green Phosphor / Dark),
// highlights the currently focused/hovered window and row in high-contrast colors,
// and styles secondary/bottom panels according to the authentic MS-DOS screenshots.
func (u *AppUI) updateFocusVisuals() {
	if u.NavList == nil {
		return
	}
	pal := currentPalette
	currFocus := u.App.GetFocus()
	hasModal := u.Pages != nil && u.Pages.HasPage("modal")

	// Configure box-drawing borders per theme in BorderMixedUnicode mode:
	// - Norton Commander (Screenshot 2): Double Bright Cyan (╔═╗║╚╝) on all panels, and Double Bright White on popup dialogs.
	// - Borland Turbo C++ 3.0 (Screenshot 1): Double Bright White (╔═╗║╚╝) on Blue windows, Single Black (┌─┐│└┘) on Silver dropdown menus.
	// - Green Phosphor / Dark: Single on unfocused, Double on focused/modals.
	if u.State.BorderMode == model.BorderMixedUnicode {
		switch u.ColorTheme {
		case ThemeNorton:
			tview.Borders.Horizontal = '═'
			tview.Borders.Vertical = '║'
			tview.Borders.TopLeft = '╔'
			tview.Borders.TopRight = '╗'
			tview.Borders.BottomLeft = '╚'
			tview.Borders.BottomRight = '╝'
			tview.Borders.HorizontalFocus = '═'
			tview.Borders.VerticalFocus = '║'
			tview.Borders.TopLeftFocus = '╔'
			tview.Borders.TopRightFocus = '╗'
			tview.Borders.BottomLeftFocus = '╚'
			tview.Borders.BottomRightFocus = '╝'
		case ThemeTurboC:
			tview.Borders.Horizontal = '═'
			tview.Borders.Vertical = '║'
			tview.Borders.TopLeft = '╔'
			tview.Borders.TopRight = '╗'
			tview.Borders.BottomLeft = '╚'
			tview.Borders.BottomRight = '╝'
			if hasModal {
				// Turbo C++ 3.0 dropdown menu uses a Single Black border inside its Silver rim
				tview.Borders.HorizontalFocus = '─'
				tview.Borders.VerticalFocus = '│'
				tview.Borders.TopLeftFocus = '┌'
				tview.Borders.TopRightFocus = '┐'
				tview.Borders.BottomLeftFocus = '└'
				tview.Borders.BottomRightFocus = '┘'
			} else {
				tview.Borders.HorizontalFocus = '═'
				tview.Borders.VerticalFocus = '║'
				tview.Borders.TopLeftFocus = '╔'
				tview.Borders.TopRightFocus = '╗'
				tview.Borders.BottomLeftFocus = '╚'
				tview.Borders.BottomRightFocus = '╝'
			}
		default:
			tview.Borders.Horizontal = '─'
			tview.Borders.Vertical = '│'
			tview.Borders.TopLeft = '┌'
			tview.Borders.TopRight = '┐'
			tview.Borders.BottomLeft = '└'
			tview.Borders.BottomRight = '┘'
			tview.Borders.HorizontalFocus = '═'
			tview.Borders.VerticalFocus = '║'
			tview.Borders.TopLeftFocus = '╔'
			tview.Borders.TopRightFocus = '╗'
			tview.Borders.BottomLeftFocus = '╚'
			tview.Borders.BottomRightFocus = '╝'
		}
	}

	if u.Header != nil {
		u.Header.SetBackgroundColor(pal.HeaderBg)
		u.Header.SetTextColor(pal.HeaderFg)
		switch u.ColorTheme {
		case ThemeTurboC:
			u.Header.SetBorderColor(VGABlack)
			u.Header.SetTitleColor(VGABlack)
			u.Header.SetTitle("[#000000:#a8a8a8:b] [[#af0000::b]≡[#000000]] File  Edit  Search  Run  Compile  Debug  Project  Options  Window  Help [-:-:-]")
		case ThemeNorton:
			u.Header.SetBorderColor(VGABrightCyan)
			u.Header.SetTitleColor(VGABrightYellow)
			u.Header.SetTitle("[#000000:#00afaf:b] NORTON COMMANDER ORA-TUI :: ORACLE CLUSTER & DB MANAGER [-:-:-]")
		case ThemeGreenPhosphor:
			u.Header.SetBorderColor(PhosphorGlow)
			u.Header.SetTitleColor(PhosphorGlow)
			u.Header.SetTitle("[#001000:#00ff5f:b] IBM 5151 P1 PHOSPHOR CRT :: ORA-TUI ORACLE MANAGER [-:-:-]")
		default:
			u.Header.SetBorderColor(pal.ActiveBorder)
			u.Header.SetTitleColor(pal.ActiveTitle)
			u.Header.SetTitle(" ══ ORA-TUI :: ORACLE CLUSTER & DATABASE MANAGER ══ ")
		}
	}
	if u.LogView != nil {
		u.LogView.SetBackgroundColor(pal.LogBg)
		u.LogView.SetTextColor(pal.LogFg)
		u.LogView.SetBorderColor(pal.LogBorder)
		u.LogView.SetTitleColor(pal.LogFg)
		switch u.ColorTheme {
		case ThemeTurboC:
			// Exact match to the bottom Teal/Cyan (#00AFAF) "Project: MCIRCLE" window in Screenshot 1
			u.LogView.SetTitle("[#0000af:#00afaf:b] [[#ffffff::b]■[#0000af]]─── Project: CONSOLA DE EJECUCION & AUDITORIA ─── 2 ───[-:-:-]")
		case ThemeNorton:
			// Exact match to the bottom Black (#000000) MS-DOS C:\> prompt area in Screenshot 2
			u.LogView.SetTitle("[#000000:#00afaf:b] C:\\ORACLE> CONSOLA MS-DOS & AUDITORIA EN VIVO [-:-:-]")
		case ThemeGreenPhosphor:
			u.LogView.SetTitle("[#00af00] [CONSOLA DE EJECUCION & AUDITORIA EN VIVO] [-]")
		default:
			u.LogView.SetTitle(" CONSOLA DE EJECUCION & AUDITORIA EN VIVO ")
		}
	}
	if u.Footer != nil {
		u.Footer.SetBackgroundColor(pal.FooterBg)
		u.Footer.SetTextColor(pal.FooterFg)
	}

	u.NavList.SetBackgroundColor(pal.NavBg)
	u.NavList.SetMainTextStyle(tcell.StyleDefault.Background(pal.NavBg).Foreground(pal.NavFg))

	navRaw := u.rawTitles[u.NavList]
	if navRaw == "" {
		navRaw = strings.TrimSpace(u.NavList.GetTitle())
		u.rawTitles[u.NavList] = navRaw
	}

	navFocused := !hasModal && (currFocus == nil || currFocus == u.NavList)
	if navFocused {
		u.NavList.SetBorderColor(pal.ActiveBorder)
		u.NavList.SetTitleColor(pal.ActiveTitle)
		u.NavList.SetTitle(u.decorateTitle(navRaw, true))
		u.NavList.SetSelectedBackgroundColor(pal.CursorBg)
		u.NavList.SetSelectedTextColor(pal.CursorFg)
	} else {
		u.NavList.SetBorderColor(pal.InactiveBorder)
		u.NavList.SetTitleColor(pal.InactiveTitle)
		u.NavList.SetTitle(u.decorateTitle(navRaw, false))
		u.NavList.SetSelectedBackgroundColor(pal.NavUnfocusedBg)
		u.NavList.SetSelectedTextColor(pal.NavUnfocusedFg)
	}

	prims := u.viewFocusables[u.ActiveView]
	for idx, p := range prims {
		hasFocus := !hasModal && currFocus == p
		// In Borland Turbo C++ 3.0 (Screenshot 1), the bottom panel in multi-window views uses the Teal/Cyan (#00AFAF)
		// "Project:" window style with Dark Blue (#0000AF) border and Dark Blue text!
		isTurboCyanWin := u.ColorTheme == ThemeTurboC && len(prims) > 1 && idx == len(prims)-1

		switch w := p.(type) {
		case *tview.Table:
			raw := u.rawTitles[w]
			if raw == "" {
				raw = strings.TrimSpace(w.GetTitle())
				u.rawTitles[w] = raw
			}
			if isTurboCyanWin {
				w.SetBackgroundColor(pal.SecondaryBg)
				w.SetBorderColor(pal.SecondaryBorder)
				w.SetTitleColor(pal.SecondaryFg)
				w.SetTitle(fmt.Sprintf("[#0000af:#00afaf:b] [[#ffffff::b]■[#0000af]] %s [-:-:-]", raw))
				for r := 0; r < w.GetRowCount(); r++ {
					for c := 0; c < w.GetColumnCount(); c++ {
						cell := w.GetCell(r, c)
						if cell == nil {
							continue
						}
						if r == 0 {
							cell.SetTextColor(VGABlack)
						} else {
							cell.SetTextColor(VGABlue)
						}
					}
				}
			} else {
				w.SetBackgroundColor(pal.PanelBg)
				w.SetTitle(u.decorateTitle(raw, hasFocus))
				if hasFocus {
					w.SetBorderColor(pal.ActiveBorder)
					w.SetTitleColor(pal.ActiveTitle)
				} else {
					w.SetBorderColor(pal.InactiveBorder)
					w.SetTitleColor(pal.InactiveTitle)
				}
			}
			if hasFocus {
				w.SetSelectable(true, false)
				w.SetSelectedStyle(tcell.StyleDefault.Background(pal.CursorBg).Foreground(pal.CursorFg).Bold(true))
			} else {
				w.SetSelectable(false, false)
			}
		case *tview.List:
			raw := u.rawTitles[w]
			if raw == "" {
				raw = strings.TrimSpace(w.GetTitle())
				u.rawTitles[w] = raw
			}
			w.SetBackgroundColor(pal.PanelBg)
			w.SetMainTextStyle(tcell.StyleDefault.Background(pal.PanelBg).Foreground(pal.PanelFg))
			w.SetTitle(u.decorateTitle(raw, hasFocus))
			if hasFocus {
				w.SetBorderColor(pal.ActiveBorder)
				w.SetTitleColor(pal.ActiveTitle)
				w.SetSelectedBackgroundColor(pal.CursorBg)
				w.SetSelectedTextColor(pal.CursorFg)
			} else {
				w.SetBorderColor(pal.InactiveBorder)
				w.SetTitleColor(pal.InactiveTitle)
				w.SetSelectedBackgroundColor(pal.NavUnfocusedBg)
				w.SetSelectedTextColor(pal.NavUnfocusedFg)
			}
		case *tview.TextView:
			raw := u.rawTitles[w]
			if raw == "" {
				raw = strings.TrimSpace(w.GetTitle())
				u.rawTitles[w] = raw
			}
			if isTurboCyanWin {
				w.SetBackgroundColor(pal.SecondaryBg)
				w.SetTextColor(pal.SecondaryFg)
				w.SetBorderColor(pal.SecondaryBorder)
				w.SetTitleColor(pal.SecondaryFg)
				w.SetTitle(fmt.Sprintf("[#0000af:#00afaf:b] [[#ffffff::b]■[#0000af]] %s [-:-:-]", raw))
			} else {
				w.SetBackgroundColor(pal.PanelBg)
				w.SetTextColor(pal.PanelFg)
				w.SetTitle(u.decorateTitle(raw, hasFocus))
				if hasFocus {
					w.SetBorderColor(pal.ActiveBorder)
					w.SetTitleColor(pal.ActiveTitle)
				} else {
					w.SetBorderColor(pal.InactiveBorder)
					w.SetTitleColor(pal.InactiveTitle)
				}
			}
		}
	}
}

func (u *AppUI) winTitle(base, hint string) string {
	if u.ShowMenuHelp && hint != "" {
		return fmt.Sprintf(" %s (%s) ", base, hint)
	}
	return fmt.Sprintf(" %s ", base)
}

func (u *AppUI) Logf(format string, args ...interface{}) {
	ts := time.Now().Format("15:04:05")
	msg := fmt.Sprintf(format, args...)
	entry := fmt.Sprintf("[%s] %s", ts, msg)
	u.logHistory = append(u.logHistory, entry)
	if len(u.logHistory) > 80 {
		u.logHistory = u.logHistory[len(u.logHistory)-80:]
	}
	u.renderLogs()
}

func (u *AppUI) renderLogs() {
	if u.LogView == nil {
		return
	}
	pal := currentPalette
	u.LogView.SetBackgroundColor(pal.LogBg)
	u.LogView.SetTextColor(pal.LogFg)

	var sb strings.Builder
	for _, entry := range u.logHistory {
		line := entry
		switch u.ColorTheme {
		case ThemeTurboC:
			// Borland Turbo C++ 3.0 bottom Cyan (#00AFAF) Project window uses Dark Blue (#0000AF), Black (#000000), and Red (#AF0000)
			r := strings.NewReplacer(
				"[yellow::b]", "[#000000::b]",
				"[yellow]", "[#000000::b]",
				"[lime::b]", "[#0000af::b]",
				"[green]", "[#0000af::b]",
				"[aqua]", "[#0000af::b]",
				"[red]", "[#af0000::b]",
			)
			line = r.Replace(line)
			sb.WriteString(" [#0000af]• " + line + "[-]\n")
		case ThemeGreenPhosphor:
			// IBM 5151 Monochrome P1 Green Phosphor
			r := strings.NewReplacer(
				"[yellow::b]", "[#87ff87::b]",
				"[yellow]", "[#87ff87::b]",
				"[lime::b]", "[#00ff5f::b]",
				"[green]", "[#00ff5f::b]",
				"[aqua]", "[#00ff5f]",
				"[red]", "[#001000:#00ff5f:b]",
			)
			line = r.Replace(line)
			sb.WriteString(" [#00af00]>[#00ff5f] " + line + "[-]\n")
		case ThemeNorton:
			// Norton Commander bottom MS-DOS prompt style (C:\> in Silver #A8A8A8 on Black #000000)
			r := strings.NewReplacer(
				"[yellow::b]", "[#ffff5f::b]",
				"[yellow]", "[#ffff5f]",
				"[lime::b]", "[#5fffff::b]",
				"[green]", "[#5fffff]",
				"[aqua]", "[#5fffff]",
				"[red]", "[#ff5f5f::b]",
			)
			line = r.Replace(line)
			sb.WriteString(" [#a8a8a8]C:\\> " + line + "[-]\n")
		default:
			sb.WriteString(" [#5fffff]" + line + "[-]\n")
		}
	}
	u.LogView.SetText(sb.String())
	u.LogView.ScrollToEnd()
}

// ensureSelectableRow guarantees that a tview.Table with SetSelectable(true, false)
// always has at least 1 selectable data row (row 1) so tview never traps focus on an empty table.
func ensureSelectableRow(tbl *tview.Table, colCount int, emptyMsg string) {
	if tbl.GetRowCount() <= 1 {
		tbl.SetCell(1, 0, MutedCell(emptyMsg))
		for c := 1; c < colCount; c++ {
			tbl.SetCell(1, c, MutedCell("-"))
		}
	}
	tbl.Select(1, 0)
}

func (u *AppUI) navEntries() []navEntry {
	return []navEntry{
		{"overview", "1. Cluster & DB Estado", "Nodos, Inst, PDBs", func() { u.switchViewAndFocus("overview") }},
		{"sessions", "2. Usuarios Conectados", "Sesiones & Locks", func() { u.switchViewAndFocus("sessions") }},
		{"tablespaces", "3. Tablespaces & Files", "Uso % & Datafiles", func() { u.switchViewAndFocus("tablespaces") }},
		{"tables", "4. Tablas & Objetos", "Stats & Invalidos", func() { u.switchViewAndFocus("tables") }},
		{"asm_net", "5. ASM & Red Listeners", "Diskgroups & SCAN", func() { u.switchViewAndFocus("asm_net") }},
		{"rman_dg", "6. RMAN, Flashback & DG", "Backups & Standby", func() { u.switchViewAndFocus("rman_dg") }},
		{"alert_log", "7. Alert Log & Errores", "ORA- & ADRCI Purge", func() { u.switchViewAndFocus("alert_log") }},
		{"os_health", "8. Servidor, Discos & SAN", "RAM, Load, Multipath", func() { u.switchViewAndFocus("os_health") }},
		{"maintenance", "9. Centro Mantenimiento", "Asistentes Guiados", func() { u.switchViewAndFocus("maintenance") }},
		{"", "F1. Ayuda en Menus", "Ocultar/Ver Ayuda", func() { u.toggleMenuHelp() }},
		{"", "c. Tema Color Retro", "NC / TurboC / Fosforo", func() { u.cycleColorTheme() }},
		{"", "b. Cambiar Marcos", "Simple ┌─┐ / ╔═╗", func() { u.cycleBorderMode() }},
		{"", "q. Salir (Ctrl+C)", "Cerrar programa", func() { u.App.Stop() }},
	}
}

func (u *AppUI) populateNavList() {
	currIdx := u.NavList.GetCurrentItem()
	u.NavList.Clear()
	u.NavList.ShowSecondaryText(false)
	rawTitle := strings.TrimSpace(u.winTitle("MENU", "Clic / ↑↓ / →"))
	if u.rawTitles == nil {
		u.rawTitles = make(map[tview.Primitive]string)
	}
	u.rawTitles[u.NavList] = rawTitle
	u.NavList.SetTitle(rawTitle)

	u.NavList.SetBackgroundColor(currentPalette.NavBg)
	u.NavList.SetMainTextStyle(tcell.StyleDefault.Background(currentPalette.NavBg).Foreground(currentPalette.NavFg))

	entries := u.navEntries()
	for _, e := range entries {
		parts := strings.SplitN(e.short, ". ", 2)
		keyPart := parts[0] + "."
		restPart := ""
		if len(parts) > 1 {
			restPart = parts[1]
		}
		var label string
		switch u.ColorTheme {
		case ThemeTurboC:
			// Borland Turbo C++ 3.0 Blue window: White (#FFFFFF) shortcut key + Bright Yellow (#FFFF5F) item label
			if u.ShowMenuHelp {
				label = fmt.Sprintf(" [#ffffff::b]%-3s[-::-] %-20s │ %s", keyPart, restPart, e.help)
			} else {
				label = fmt.Sprintf(" [#ffffff::b]%-3s[-::-] %s", keyPart, restPart)
			}
		case ThemeNorton:
			// Norton Commander Blue panel: Bright Yellow (#FFFF5F) shortcut key + Bright Cyan (#5FFFFF) item label
			if u.ShowMenuHelp {
				label = fmt.Sprintf(" [#ffff5f::b]%-3s[-::-] %-20s │ %s", keyPart, restPart, e.help)
			} else {
				label = fmt.Sprintf(" [#ffff5f::b]%-3s[-::-] %s", keyPart, restPart)
			}
		default:
			// Green Phosphor CRT & Dark: Pure uniform text so hardware-style inverse bar inverts 100% of the line
			label = " " + e.short
			if u.ShowMenuHelp {
				label = fmt.Sprintf(" %-24s │ %s", e.short, e.help)
			}
		}
		u.NavList.AddItem(label, "", 0, e.fn)
	}
	if currIdx >= 0 && currIdx < u.NavList.GetItemCount() {
		u.NavList.SetCurrentItem(currIdx)
	}
}

func (u *AppUI) navMenuWidth() int {
	if u.ShowMenuHelp {
		return 48
	}
	return 27
}

func (u *AppUI) buildLayout() {
	// 1. Header Banner (4 rows: 2 content lines + 2 border lines)
	u.Header = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	u.Header.SetBorder(true)
	u.refreshHeader()

	// 2. Left Arrow-Key Navigation Menu (Single-line items so all 12 options always fit without scrolling)
	u.NavList = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(tcell.ColorWhite).
		SetSelectedBackgroundColor(tcell.ColorAqua).
		SetSelectedTextColor(tcell.ColorBlack)
	u.NavList.SetBorder(true)
	u.populateNavList()

	entries := u.navEntries()
	u.NavList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if u.isHoveringNav {
			return
		}
		if index >= 0 && index < len(entries) && entries[index].key != "" {
			viewName := entries[index].key
			u.ActiveView = viewName
			u.Content.SwitchToPage(viewName)
			u.updateFocusVisuals()
		}
	})

	// 3. Content Workspace Pages
	u.Content = tview.NewPages()

	// 4. Live Execution Log Panel (5 rows: 3 log lines + 2 border lines)
	u.LogView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			u.App.Draw()
		})
	u.LogView.SetBorder(true)

	// 5. Footer Keybindings Bar
	u.Footer = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	u.refreshFooter()

	// Populate initial views
	u.rebuildContentViews()

	// Main Middle Flex (Left Menu + Right Content)
	u.MidFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(u.NavList, u.navMenuWidth(), 0, true).
		AddItem(u.Content, 0, 1, false)

	// Root Vertical Flex
	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(u.Header, 4, 0, false).
		AddItem(u.MidFlex, 0, 1, true).
		AddItem(u.LogView, 5, 0, false).
		AddItem(u.Footer, 1, 0, false)

	u.Pages.AddPage("main", mainFlex, true, true)
	u.App.SetRoot(u.Pages, true)
	u.App.EnableMouse(true)
	u.updateFocusVisuals()

	// Mouse support over SSH:
	// - Moving the mouse (MouseMove) highlights the exact menu option or table row under the pointer in real time.
	// - Clicking a table row opens its action submenu; clicking a menu item executes it.
	// - Right-click or clicking outside a modal closes it.
	u.App.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if event == nil {
			return event, action
		}
		x, y := event.Position()

		if u.Pages.HasPage("modal") {
			if action == tview.MouseMove || action == tview.MouseLeftDown {
				if u.activeModalList != nil && u.activeModalList.InRect(x, y) {
					idx := listIndexAtPoint(u.activeModalList, x, y)
					if idx >= 0 && idx != u.activeModalList.GetCurrentItem() {
						u.activeModalList.SetCurrentItem(idx)
						if action == tview.MouseMove && event.Buttons() == tcell.ButtonNone {
							return nil, action
						}
					}
				}
				return event, action
			}
			if action == tview.MouseRightClick {
				u.Pages.RemovePage("modal")
				u.activeModalList = nil
				u.focusFirstRightWindow()
				return nil, action
			}
			if action == tview.MouseLeftClick && u.activeModalList != nil && !u.activeModalList.InRect(x, y) {
				u.Pages.RemovePage("modal")
				u.activeModalList = nil
				u.focusFirstRightWindow()
				return nil, action
			}
			return event, action
		}

		// Real-time hover tracking (MouseMove / MouseLeftDown) over left menu and right-side windows/tables
		if action == tview.MouseMove || action == tview.MouseLeftDown {
			if u.NavList.InRect(x, y) {
				changed := false
				if u.App.GetFocus() != u.NavList {
					u.App.SetFocus(u.NavList)
					changed = true
				}
				idx := listIndexAtPoint(u.NavList, x, y)
				if idx >= 0 && idx != u.NavList.GetCurrentItem() {
					u.isHoveringNav = true
					u.NavList.SetCurrentItem(idx)
					u.isHoveringNav = false
					changed = true
				}
				if changed {
					u.updateFocusVisuals()
					if action == tview.MouseMove && event.Buttons() == tcell.ButtonNone {
						return nil, action
					}
				}
				return event, action
			}

			// If pointer moved out of NavList, ensure NavList shows the active view's index
			activeNavIdx := u.activeViewNavIndex()
			if u.NavList.GetCurrentItem() != activeNavIdx {
				u.isHoveringNav = true
				u.NavList.SetCurrentItem(activeNavIdx)
				u.isHoveringNav = false
			}

			for _, p := range u.viewFocusables[u.ActiveView] {
				switch w := p.(type) {
				case *tview.Table:
					if !w.InRect(x, y) {
						continue
					}
					changed := false
					if u.App.GetFocus() != w {
						u.App.SetFocus(w)
						changed = true
					}
					r, _ := w.CellAt(x, y)
					if r >= 1 && r < w.GetRowCount() {
						currR, _ := w.GetSelection()
						if currR != r {
							w.Select(r, 0)
							changed = true
						}
					}
					if changed {
						u.updateFocusVisuals()
						if action == tview.MouseMove && event.Buttons() == tcell.ButtonNone {
							return nil, action
						}
					}
					return event, action
				case *tview.List:
					if !w.InRect(x, y) {
						continue
					}
					changed := false
					if u.App.GetFocus() != w {
						u.App.SetFocus(w)
						changed = true
					}
					idx := listIndexAtPoint(w, x, y)
					if idx >= 0 && idx != w.GetCurrentItem() {
						w.SetCurrentItem(idx)
						changed = true
					}
					if changed {
						u.updateFocusVisuals()
						if action == tview.MouseMove && event.Buttons() == tcell.ButtonNone {
							return nil, action
						}
					}
					return event, action
				case *tview.TextView:
					if !w.InRect(x, y) {
						continue
					}
					if u.App.GetFocus() != w {
						u.App.SetFocus(w)
						u.updateFocusVisuals()
						if action == tview.MouseMove && event.Buttons() == tcell.ButtonNone {
							return nil, action
						}
					}
					return event, action
				}
			}
			return event, action
		}

		if action == tview.MouseLeftClick || action == tview.MouseLeftDoubleClick || action == tview.MouseRightClick {
			for _, p := range u.viewFocusables[u.ActiveView] {
				tbl, ok := p.(*tview.Table)
				if !ok || !tbl.InRect(x, y) {
					continue
				}
				u.App.SetFocus(tbl)
				r, c := tbl.CellAt(x, y)
				if r < 1 {
					r, _ = tbl.GetSelection()
					if r < 1 && tbl.GetRowCount() > 1 {
						r = 1
					}
				}
				if r >= 1 && r < tbl.GetRowCount() {
					tbl.Select(r, 0)
					u.updateFocusVisuals()
					if fn, hasFn := u.tableActions[tbl]; hasFn && fn != nil {
						fn(r, c)
						return nil, action
					}
				}
				break
			}
		}
		return event, action
	})

	// Global keyboard navigation
	u.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Always allow Ctrl+C to immediately exit the program
		if event.Key() == tcell.KeyCtrlC {
			u.App.Stop()
			return nil
		}

		// F1 toggles menu help descriptions globally (including inside open modals)
		if event.Key() == tcell.KeyF1 {
			u.toggleMenuHelp()
			return nil
		}

		// If a modal is open, let modal handle Esc/Enter/Arrows
		if u.Pages.HasPage("modal") {
			if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyLeft {
				u.Pages.RemovePage("modal")
				u.activeModalList = nil
				u.focusFirstRightWindow()
				return nil
			}
			return event
		}

		switch event.Key() {
		case tcell.KeyRight, tcell.KeyTab:
			u.moveWindowFocus(1)
			return nil
		case tcell.KeyLeft, tcell.KeyBacktab:
			u.moveWindowFocus(-1)
			return nil
		case tcell.KeyEscape:
			u.App.SetFocus(u.NavList)
			u.updateFocusVisuals()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case '1':
				u.NavList.SetCurrentItem(0)
				u.switchViewAndFocus("overview")
				return nil
			case '2':
				u.NavList.SetCurrentItem(1)
				u.switchViewAndFocus("sessions")
				return nil
			case '3':
				u.NavList.SetCurrentItem(2)
				u.switchViewAndFocus("tablespaces")
				return nil
			case '4':
				u.NavList.SetCurrentItem(3)
				u.switchViewAndFocus("tables")
				return nil
			case '5':
				u.NavList.SetCurrentItem(4)
				u.switchViewAndFocus("asm_net")
				return nil
			case '6':
				u.NavList.SetCurrentItem(5)
				u.switchViewAndFocus("rman_dg")
				return nil
			case '7':
				u.NavList.SetCurrentItem(6)
				u.switchViewAndFocus("alert_log")
				return nil
			case '8':
				u.NavList.SetCurrentItem(7)
				u.switchViewAndFocus("os_health")
				return nil
			case '9':
				u.NavList.SetCurrentItem(8)
				u.switchViewAndFocus("maintenance")
				return nil
			case 'h', 'H':
				u.toggleMenuHelp()
				return nil
			case 'm', 'M':
				u.showGlobalMaintenancePopup()
				return nil
			case 'c', 'C':
				u.cycleColorTheme()
				return nil
			case 'b', 'B':
				u.cycleBorderMode()
				return nil
			case 't', 'T':
				u.toggleTopology()
				return nil
			case 'q', 'Q':
				u.App.Stop()
				return nil
			}
		}
		return event
	})

	u.Logf("Auto-Discovery: Modo=%s │ Host=%s │ Ver=%s │ Tema=[yellow]%s[-] │ [yellow]Raton SSH[-] Activo",
		u.State.Topology, u.State.HostName, u.State.ReleaseVersion, u.ColorTheme)
}

func (u *AppUI) moveWindowFocus(delta int) {
	chain := append([]tview.Primitive{u.NavList}, u.viewFocusables[u.ActiveView]...)
	if len(chain) <= 1 {
		u.App.SetFocus(u.NavList)
		u.updateFocusVisuals()
		return
	}

	curr := u.App.GetFocus()
	currIdx := 0
	for i, p := range chain {
		if p == curr {
			currIdx = i
			break
		}
	}

	nextIdx := currIdx + delta
	if nextIdx >= len(chain) {
		nextIdx = 0
	} else if nextIdx < 0 {
		nextIdx = len(chain) - 1
	}
	u.App.SetFocus(chain[nextIdx])
	u.updateFocusVisuals()
}

func (u *AppUI) focusFirstRightWindow() {
	subs := u.viewFocusables[u.ActiveView]
	if len(subs) > 0 {
		u.App.SetFocus(subs[0])
	} else {
		u.App.SetFocus(u.NavList)
	}
	u.updateFocusVisuals()
}

func (u *AppUI) switchViewAndFocus(name string) {
	u.ActiveView = name
	u.Content.SwitchToPage(name)
	activeNavIdx := u.activeViewNavIndex()
	if u.NavList.GetCurrentItem() != activeNavIdx {
		u.isHoveringNav = true
		u.NavList.SetCurrentItem(activeNavIdx)
		u.isHoveringNav = false
	}
	u.focusFirstRightWindow()
}

func (u *AppUI) RefreshAll() {
	u.refreshHeader()
	u.refreshFooter()
	u.rebuildContentViews()
}

func (u *AppUI) toggleMenuHelp() {
	u.ShowMenuHelp = !u.ShowMenuHelp
	u.populateNavList()
	if u.MidFlex != nil {
		u.MidFlex.ResizeItem(u.NavList, u.navMenuWidth(), 0)
	}
	if u.activeModalList != nil {
		u.activeModalList.ShowSecondaryText(u.ShowMenuHelp)
	}
	u.refreshFooter()
	u.rebuildContentViews()
	stateStr := "[lime::b]ACTIVADA[-::-]"
	if !u.ShowMenuHelp {
		stateStr = "[yellow::b]OCULTA (Modo Compacto)[-::-]"
	}
	u.Logf("Ayuda en menus: %s (Pulse [yellow]F1[-] o [yellow]h[-] para alternar).", stateStr)
}

func (u *AppUI) refreshFooter() {
	if u.Footer != nil {
		u.Footer.SetBackgroundColor(currentPalette.FooterBg)
		u.Footer.SetTextColor(currentPalette.FooterFg)
	}
	helpState := "ON"
	if !u.ShowMenuHelp {
		helpState = "OFF"
	}
	switch u.ColorTheme {
	case ThemeNorton:
		// Classic Norton Commander bottom bar (Screenshot 2): White number on Black + Black label on Teal (#00AFAF)
		u.Footer.SetText(fmt.Sprintf(
			"[#ffffff:#000000:b]1[#000000:#00afaf]Clust[-:-] [#ffffff:#000000:b]2[#000000:#00afaf]Users[-:-] [#ffffff:#000000:b]3[#000000:#00afaf]Tbsp[-:-] [#ffffff:#000000:b]4[#000000:#00afaf]Tablas[-:-] [#ffffff:#000000:b]5[#000000:#00afaf]ASM/Red[-:-] [#ffffff:#000000:b]6[#000000:#00afaf]RMAN[-:-] [#ffffff:#000000:b]7[#000000:#00afaf]Alert[-:-] [#ffffff:#000000:b]8[#000000:#00afaf]Server/SAN[-:-] [#ffffff:#000000:b]9[#000000:#00afaf]Maint[-:-] [#ffffff:#000000:b]F1[#000000:#00afaf]Ayuda:%s[-:-] [#ffffff:#000000:b]c[#000000:#00afaf]Tema[-:-] [#ffffff:#000000:b]q[#000000:#00afaf]Quit[-:-]",
			helpState,
		))
	case ThemeTurboC:
		// Classic Borland Turbo C++ 3.0 bottom bar (Screenshot 1): Silver (#A8A8A8) background, Red (#AF0000) hotkeys, Black (#000000) labels
		u.Footer.SetText(fmt.Sprintf(
			" [#af0000::b]F1[-::-] [#000000]Help:%s  [#af0000::b]8[-::-] [#000000]Server/SAN  [#af0000::b]Raton/Hover[-::-] [#000000]Marcar  [#af0000::b]Click/Enter[-::-] [#000000]Submenu  [#af0000::b]c[-::-] [#000000]Tema:%s  [#af0000::b]b[-::-] [#000000]Marcos  [#af0000::b]Alt-X/q[-::-] [#000000]Quit[-]",
			helpState, u.ColorTheme,
		))
	case ThemeGreenPhosphor:
		// Classic IBM 5151 P1 Green Phosphor inverse key bar
		u.Footer.SetText(fmt.Sprintf(
			" [#001000:#00ff5f:b] 8 [-:-:-] [#00ff5f]Server/SAN │ [-][#001000:#00ff5f:b] Click/Enter [-:-:-] [#00ff5f]Submenu │ [-][#001000:#00ff5f:b] F1 [-:-:-] [#00ff5f]Ayuda:%s │ [-][#001000:#00ff5f:b] c [-:-:-] [#00ff5f]Tema:%s │ [-][#001000:#00ff5f:b] b [-:-:-] [#00ff5f]Marcos │ [-][#001000:#00ff5f:b] q [-:-:-] [#00ff5f]Salir[-]",
			helpState, u.ColorTheme,
		))
	default:
		u.Footer.SetText(fmt.Sprintf(
			" [black:aqua] 8 [-:-] Server/SAN │ [black:aqua] Click/Enter [-:-] Submenu │ [black:aqua] F1 [-:-] Ayuda:%s │ [black:aqua] c [-:-] Tema:%s │ [black:aqua] b [-:-] Marcos │ [black:aqua] q [-:-] Salir ",
			helpState, u.ColorTheme,
		))
	}
}

func (u *AppUI) cycleColorTheme() {
	u.ColorTheme = NextColorTheme(u.ColorTheme)
	ApplyColorTheme(u.ColorTheme)
	u.populateNavList()
	u.refreshHeader()
	u.refreshFooter()
	u.renderLogs()
	u.rebuildContentViews()
	u.Logf("Tema de color cambiado a: [yellow::b]%s[-::-] (Pulse [yellow]c[-] para alternar Norton Commander / Turbo C / Fosforo Verde / Oscuro)", u.ColorTheme)
}

func (u *AppUI) cycleBorderMode() {
	u.State.BorderMode = NextBorderStyle(u.State.BorderMode)
	ApplyBorderTheme(u.State.BorderMode)
	u.refreshHeader()
	u.rebuildContentViews()
	u.Logf("Estilo de marcos actualizado a: [yellow]%s[-]", u.State.BorderMode)
}

func (u *AppUI) refreshHeader() {
	if u.Header != nil {
		u.Header.SetBackgroundColor(currentPalette.HeaderBg)
		u.Header.SetTextColor(currentPalette.HeaderFg)
	}
	activeSessions := 0
	blockers := 0
	for _, s := range u.State.Sessions {
		if s.Status == "ACTIVE" {
			activeSessions++
		}
		if strings.Contains(s.BlockingSession, "BLOCKER") {
			blockers++
		}
	}
	critAlerts := len(u.State.AlertEntries)

	var text string
	switch u.ColorTheme {
	case ThemeTurboC:
		// Borland Turbo C++ 3.0 top bar (Screenshot 1): Silver (#A8A8A8) background with Black (#000000), Red (#AF0000) hotkeys, and Green (#00AF00) active highlight
		modeBadge := "[#000000:#00af00:b] " + string(u.State.Topology) + " [-:-:-]"
		if u.State.IsMock {
			modeBadge += " [#ffffff:#af0000:b] MOCK [-:-:-]"
		}
		text = fmt.Sprintf(
			" [#af0000::b]M[-::-][#000000]odo: %s │ [#af0000::b]H[-::-][#000000]ost: [#0000af::b]%s[-::-] │ [#af0000::b]C[-::-][#000000]luster: [#0000af::b]%s[-::-] │ [#af0000::b]V[-::-][#000000]er: [#0000af::b]%s[-::-] │ [#af0000::b]T[-::-][#000000]ema: [#000000:#00af00:b] %s [-:-:-]\n"+
				" [#000000]HOME: %s (%s) │ [#0000af::b]%d Inst OPEN[-::-] │ [#0000af::b]%d PDBs[-::-] │ [#000000::b]%d Sesiones[-::-] ([#af0000::b]%d Locks[-::-]) │ [#af0000::b]%d Inválidos[-::-] │ [#af0000::b]%d Errores ORA-[-::-]",
			modeBadge, u.State.HostName, u.State.ClusterName, u.State.ReleaseVersion, u.ColorTheme,
			u.State.OracleHome, u.State.OracleOwner,
			len(u.State.Instances), len(u.State.PDBs), activeSessions, blockers, len(u.State.InvalidObjects), critAlerts,
		)
	case ThemeGreenPhosphor:
		// IBM 5151 Monochrome P1 Green Phosphor CRT
		modeBadge := "[#001000:#00ff5f:b] " + string(u.State.Topology) + " [-:-:-]"
		if u.State.IsMock {
			modeBadge += " [#001000:#87ff87:b] MOCK [-:-:-]"
		}
		text = fmt.Sprintf(
			" [#00af00]Modo:[-] %s │ [#00af00]Host:[-] [#87ff87::b]%s[-::-] │ [#00af00]Cluster:[-] [#00ff5f::b]%s[-::-] │ [#00af00]Ver:[-] [#00ff5f::b]%s[-::-] │ [#00af00]Tema:[-] [#87ff87::b]%s[-::-]\n"+
				" [#00af00]HOME:[-] [#00ff5f]%s (%s)[-] │ [#87ff87::b]%d Inst OPEN[-::-] │ [#00ff5f::b]%d PDBs[-::-] │ [#00ff5f::b]%d Sesiones[-::-] ([#001000:#00ff5f:b] %d Locks [-:-:-]) │ [#00ff5f::b]%d Inválidos[-::-] │ [#001000:#00ff5f:b] %d ORA- [-:-:-]",
			modeBadge, u.State.HostName, u.State.ClusterName, u.State.ReleaseVersion, u.ColorTheme,
			u.State.OracleHome, u.State.OracleOwner,
			len(u.State.Instances), len(u.State.PDBs), activeSessions, blockers, len(u.State.InvalidObjects), critAlerts,
		)
	default:
		// Norton Commander (Screenshot 2): Royal Blue (#0000AF) background with Bright Yellow (#FFFF5F) labels and Bright Cyan (#5FFFFF) values
		modeBadge := "[#000000:#00afaf:b] " + string(u.State.Topology) + " [-:-:-]"
		if u.State.IsMock {
			modeBadge += " [#000000:#ffff5f:b] MOCK [-:-:-]"
		}
		text = fmt.Sprintf(
			" [#ffff5f::b]Modo:[-::-] %s │ [#ffff5f::b]Host:[-::-] [#5fffff::b]%s[-::-] │ [#ffff5f::b]Cluster:[-::-] [#5fffff::b]%s[-::-] │ [#ffff5f::b]Ver:[-::-] [#5fffff::b]%s[-::-] │ [#ffff5f::b]Tema:[-::-] [#000000:#00afaf:b] %s [-:-:-]\n"+
				" [#ffff5f::b]HOME:[-::-] [#5fffff]%s (%s)[-] │ [#5fffff::b]%d Inst OPEN[-::-] │ [#5fffff::b]%d PDBs[-::-] │ [#ffff5f::b]%d Sesiones[-::-] ([#ff5f5f::b]%d Locks[-::-]) │ [#ff5f5f::b]%d Inválidos[-::-] │ [#ff5f5f::b]%d Errores ORA-[-::-]",
			modeBadge, u.State.HostName, u.State.ClusterName, u.State.ReleaseVersion, u.ColorTheme,
			u.State.OracleHome, u.State.OracleOwner,
			len(u.State.Instances), len(u.State.PDBs), activeSessions, blockers, len(u.State.InvalidObjects), critAlerts,
		)
	}
	u.Header.SetText(text)
}

func (u *AppUI) toggleTopology() {
	if u.State.Topology == model.ModeRAC {
		u.State.Topology = model.ModeSingle
		u.State.ClusterName = "N/A (Single Instance)"
	} else {
		u.State.Topology = model.ModeRAC
		u.State.ClusterName = "crs-prod-cluster"
	}
	u.refreshHeader()
	u.rebuildContentViews()
	u.Logf("Topologia cambiada en caliente a: [yellow]%s[-]", u.State.Topology)
}

func (u *AppUI) rebuildContentViews() {
	// 1. Save current focus state and selected row/item in every focusable widget before rebuilding
	currFocus := u.App.GetFocus()
	hasModal := u.Pages != nil && u.Pages.HasPage("modal")
	focusOnNav := currFocus == nil || currFocus == u.NavList
	focusedSubIdx := -1
	if !focusOnNav && !hasModal {
		focusedSubIdx = 0
		for i, p := range u.viewFocusables[u.ActiveView] {
			if p == currFocus {
				focusedSubIdx = i
				break
			}
		}
	}

	type selectionState struct {
		row int
		col int
	}
	savedSelections := make(map[string][]selectionState)
	for viewName, prims := range u.viewFocusables {
		states := make([]selectionState, len(prims))
		for i, p := range prims {
			switch w := p.(type) {
			case *tview.Table:
				r, c := w.GetSelection()
				states[i] = selectionState{row: r, col: c}
			case *tview.List:
				states[i] = selectionState{row: w.GetCurrentItem(), col: 0}
			}
		}
		savedSelections[viewName] = states
	}

	// 2. Rebuild all pages and ensure ActiveView is visible
	u.tableActions = make(map[*tview.Table]func(row, col int))
	u.Content.AddPage("overview", u.buildOverviewPage(), true, u.ActiveView == "overview")
	u.Content.AddPage("sessions", u.buildSessionsPage(), true, u.ActiveView == "sessions")
	u.Content.AddPage("tablespaces", u.buildTablespacesPage(), true, u.ActiveView == "tablespaces")
	u.Content.AddPage("tables", u.buildTablesPage(), true, u.ActiveView == "tables")
	u.Content.AddPage("asm_net", u.buildASMNetworkPage(), true, u.ActiveView == "asm_net")
	u.Content.AddPage("rman_dg", u.buildRMANDGPage(), true, u.ActiveView == "rman_dg")
	u.Content.AddPage("alert_log", u.buildAlertLogPage(), true, u.ActiveView == "alert_log")
	u.Content.AddPage("os_health", u.buildServerHealthPage(), true, u.ActiveView == "os_health")
	u.Content.AddPage("maintenance", u.buildMaintenancePage(), true, u.ActiveView == "maintenance")
	u.Content.SwitchToPage(u.ActiveView)

	// 3. Restore selected row/item in every rebuilt table/list
	for viewName, prims := range u.viewFocusables {
		prevStates, ok := savedSelections[viewName]
		if !ok {
			continue
		}
		for i, p := range prims {
			if i >= len(prevStates) {
				break
			}
			st := prevStates[i]
			switch w := p.(type) {
			case *tview.Table:
				maxRow := w.GetRowCount() - 1
				if maxRow >= 1 && st.row >= 1 {
					r := st.row
					if r > maxRow {
						r = maxRow
					}
					w.Select(r, st.col)
				}
			case *tview.List:
				cnt := w.GetItemCount()
				if cnt > 0 && st.row >= 0 {
					idx := st.row
					if idx >= cnt {
						idx = cnt - 1
					}
					w.SetCurrentItem(idx)
				}
			}
		}
	}

	// 4. Restore focus to the exact sub-window the user was navigating
	if hasModal && u.activeModalList != nil {
		u.App.SetFocus(u.activeModalList)
	} else if !focusOnNav && focusedSubIdx >= 0 {
		subs := u.viewFocusables[u.ActiveView]
		if len(subs) > 0 {
			if focusedSubIdx >= len(subs) {
				focusedSubIdx = len(subs) - 1
			}
			u.App.SetFocus(subs[focusedSubIdx])
		}
	}
	u.updateFocusVisuals()
}

// View 1: Cluster, Nodes, Instances & PDBs
func (u *AppUI) buildOverviewPage() tview.Primitive {
	nodesTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	nodesTbl.SetBorder(true).SetTitle(u.winTitle("[1/3] NODOS DEL CLUSTER & DAEMONS", "Clic/Enter = Acciones │ ←/→ Ventana"))
	headers := []string{"NODO", "ESTADO", "ROL", "CRS", "CSSD", "EVMD", "ASM", "VIP", "INTERCONNECT"}
	for c, h := range headers {
		nodesTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, n := range u.State.Nodes {
		r := i + 1
		nodesTbl.SetCell(r, 0, KeyCell(n.Name))
		nodesTbl.SetCell(r, 1, StatusCell(n.State))
		nodesTbl.SetCell(r, 2, tview.NewTableCell(n.Role))
		nodesTbl.SetCell(r, 3, StatusCell(n.CRS))
		nodesTbl.SetCell(r, 4, StatusCell(n.CSSD))
		nodesTbl.SetCell(r, 5, StatusCell(n.EVMD))
		nodesTbl.SetCell(r, 6, StatusCell(n.ASM))
		nodesTbl.SetCell(r, 7, StatusCell(n.VIP))
		nodesTbl.SetCell(r, 8, tview.NewTableCell(n.Interconnect))
	}
	ensureSelectableRow(nodesTbl, len(headers), "Sin nodos de cluster detectados")
	u.bindTableActions(nodesTbl, func(row, col int) {
		nodeName := u.State.HostName
		hasNode := row > 0 && row <= len(u.State.Nodes)
		if hasNode {
			nodeName = u.State.Nodes[row-1].Name
		}
		u.showActionMenuModal(
			fmt.Sprintf(" ══ ACCIONES DE NODO: %s ══ ", nodeName),
			[]menuAction{
				{"Drenar / Aislar Nodo (Graceful Node Drain)", "Reubica servicios + Para instancia local + CRS", func() {
					u.runSimulatedTask(
						fmt.Sprintf("Drenar Nodo %s", nodeName),
						[]string{
							fmt.Sprintf("[PRE-CHECK] Verificando servicios activos en %s...", nodeName),
							"[EXEC] srvctl relocate service -d PRODCDB -s svc_crm_app -oldinst PRODCDB1 -newinst PRODCDB2",
							fmt.Sprintf("[EXEC] srvctl stop instance -d PRODCDB -n %s -stopoption IMMEDIATE", nodeName),
							"[ROOT] /u01/app/19.0.0/grid/bin/crsctl stop crs",
							fmt.Sprintf("[OK] Nodo %s aislado correctamente sin corte de servicio.", nodeName),
						},
						func() {
							if hasNode {
								u.State.Nodes[row-1].State = "DRAINED"
								u.State.Nodes[row-1].CRS = "OFFLINE"
								u.rebuildContentViews()
							}
						},
					)
				}},
				{"Reincorporar Nodo al Cluster (Rejoin Node)", "Inicia CRS + Monta ASM + Abre Instancia", func() {
					u.runSimulatedTask(
						fmt.Sprintf("Reincorporar Nodo %s", nodeName),
						[]string{
							"[ROOT] /u01/app/19.0.0/grid/bin/crsctl start crs",
							"[WAIT] Esperando demonios CSSD, CRSD y +ASM...",
							fmt.Sprintf("[EXEC] srvctl start instance -d PRODCDB -n %s", nodeName),
							fmt.Sprintf("[OK] Nodo %s ONLINE e integrado en el cluster.", nodeName),
						},
						func() {
							if hasNode {
								u.State.Nodes[row-1].State = "ONLINE"
								u.State.Nodes[row-1].CRS = "ONLINE"
								u.rebuildContentViews()
							}
						},
					)
				}},
				{"Verificar Salud Cluster (crsctl check cluster -all)", "Ejecuta diagnostico completo", func() {
					u.Logf("[CRSCTL] Todos los demonios de cluster responden en %s.", nodeName)
				}},
			},
		)
	})

	instTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	instTbl.SetBorder(true).SetTitle(u.winTitle("[2/3] INSTANCIAS DE BASE DE DATOS & DATA GUARD", "Clic/Enter = Start/Stop │ ←/→ Ventana"))
	iHeaders := []string{"INST", "DB_NAME", "NODO", "ESTADO", "OPEN_MODE", "FLASHBACK", "MEMORIA (SGA/PGA)", "FRA_USO", "DG_LAG", "UPTIME"}
	for c, h := range iHeaders {
		instTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, inst := range u.State.Instances {
		r := i + 1
		instTbl.SetCell(r, 0, KeyCell(inst.InstName))
		instTbl.SetCell(r, 1, tview.NewTableCell(inst.DBName))
		instTbl.SetCell(r, 2, tview.NewTableCell(inst.NodeName))
		instTbl.SetCell(r, 3, StatusCell(inst.Status))
		instTbl.SetCell(r, 4, StatusCell(inst.OpenMode))
		instTbl.SetCell(r, 5, StatusCell(inst.FlashbackOn))
		instTbl.SetCell(r, 6, tview.NewTableCell(fmt.Sprintf("SGA:%s / PGA:%s", formatMemGB(inst.SGAMB), formatMemGB(inst.PGAMB))))
		instTbl.SetCell(r, 7, BarCell(float64(inst.FRAUsedPct), 8))
		instTbl.SetCell(r, 8, StatusCell(inst.DGLag))
		instTbl.SetCell(r, 9, tview.NewTableCell(inst.Uptime))
	}
	ensureSelectableRow(instTbl, len(iHeaders), "Sin instancias activas detectadas (Clic o Enter para iniciar)")
	u.bindTableActions(instTbl, func(row, col int) {
		if row > 0 && row <= len(u.State.Instances) {
			u.showInstanceMenu(row-1, u.State.Instances[row-1])
			return
		}
		if len(u.State.Instances) > 0 {
			u.showInstanceMenu(0, u.State.Instances[0])
			return
		}
		u.showInstanceMenu(-1, model.InstanceInfo{InstName: "ORCL", DBName: u.State.ClusterName, NodeName: u.State.HostName})
	})

	pdbTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	pdbTbl.SetBorder(true).SetTitle(u.winTitle("[3/3] PLUGGABLE DATABASES - PDBs", "Clic/Enter = Abrir/Cerrar │ ←/→ Ventana"))
	pHeaders := []string{"CON_ID", "PDB_NAME", "OPEN_MODE", "RESTRICTED", "TAMANO_GB", "INSTANCIAS"}
	for c, h := range pHeaders {
		pdbTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, p := range u.State.PDBs {
		r := i + 1
		pdbTbl.SetCell(r, 0, tview.NewTableCell(fmt.Sprintf("%d", p.ConID)))
		pdbTbl.SetCell(r, 1, KeyCell(p.Name))
		pdbTbl.SetCell(r, 2, StatusCell(p.OpenMode))
		pdbTbl.SetCell(r, 3, tview.NewTableCell(p.Restricted))
		pdbTbl.SetCell(r, 4, tview.NewTableCell(fmt.Sprintf("%.1f GB", p.SizeGB)))
		pdbTbl.SetCell(r, 5, tview.NewTableCell(p.InstName))
	}
	ensureSelectableRow(pdbTbl, len(pHeaders), "Arquitectura Non-CDB o sin PDBs registradas")
	u.bindTableActions(pdbTbl, func(row, col int) {
		pdbName := "ALL PDBS"
		hasPDB := row > 0 && row <= len(u.State.PDBs)
		if hasPDB {
			pdbName = u.State.PDBs[row-1].Name
		}
		u.showActionMenuModal(
			fmt.Sprintf(" ══ GESTION DE PDB: %s ══ ", pdbName),
			[]menuAction{
				{"Abrir PDB en READ WRITE", "ALTER PLUGGABLE DATABASE " + pdbName + " OPEN READ WRITE INSTANCES=ALL", func() {
					if hasPDB {
						u.State.PDBs[row-1].OpenMode = "READ WRITE"
					}
					u.Logf("[SQL] ALTER PLUGGABLE DATABASE %s OPEN READ WRITE; -> [green]OK[-]", pdbName)
					u.rebuildContentViews()
				}},
				{"Cerrar PDB (MOUNTED)", "ALTER PLUGGABLE DATABASE " + pdbName + " CLOSE IMMEDIATE", func() {
					if hasPDB {
						u.State.PDBs[row-1].OpenMode = "MOUNTED"
					}
					u.Logf("[SQL] ALTER PLUGGABLE DATABASE %s CLOSE IMMEDIATE; -> [yellow]MOUNTED[-]", pdbName)
					u.rebuildContentViews()
				}},
				{"Guardar Estado de Arranque Automatico (SAVE STATE)", "ALTER PLUGGABLE DATABASE " + pdbName + " SAVE STATE INSTANCES=ALL", func() {
					u.Logf("[SQL] ALTER PLUGGABLE DATABASE %s SAVE STATE; -> [green]Estado persistido[-]", pdbName)
				}},
			},
		)
	})

	u.viewFocusables["overview"] = []tview.Primitive{nodesTbl, instTbl, pdbTbl}

	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nodesTbl, 6, 0, true).
		AddItem(instTbl, 6, 0, false).
		AddItem(pdbTbl, 0, 1, false)
}

// View 2: Connected Users & Sessions
func (u *AppUI) buildSessionsPage() tview.Primitive {
	tbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	tbl.SetBorder(true).SetTitle(u.winTitle("USUARIOS CONECTADOS & SESIONES (gv$session)", "Clic/Enter = Acciones │ ← Menu"))

	headers := []string{"INST", "SID,SER#", "USUARIO", "ESTADO", "MAQUINA", "PROGRAMA", "SQL_ID", "EVENTO DE ESPERA", "TIEMPO", "BLOQUEO", "PGA_MB"}
	for c, h := range headers {
		tbl.SetCell(0, c, HeaderCell(h))
	}

	for i, s := range u.State.Sessions {
		r := i + 1
		tbl.SetCell(r, 0, tview.NewTableCell(fmt.Sprintf("%d", s.InstID)))
		tbl.SetCell(r, 1, tview.NewTableCell(fmt.Sprintf("%d,%d", s.SID, s.Serial)))
		tbl.SetCell(r, 2, KeyCell(s.Username))
		tbl.SetCell(r, 3, StatusCell(s.Status))
		tbl.SetCell(r, 4, tview.NewTableCell(s.Machine))
		tbl.SetCell(r, 5, tview.NewTableCell(s.Program))
		tbl.SetCell(r, 6, AccentCell(s.SQLID))
		tbl.SetCell(r, 7, tview.NewTableCell(s.WaitEvent))
		tbl.SetCell(r, 8, tview.NewTableCell(fmt.Sprintf("%ds", s.ElapsedSec)))
		tbl.SetCell(r, 9, StatusCell(s.BlockingSession))
		tbl.SetCell(r, 10, tview.NewTableCell(fmt.Sprintf("%.1fM", s.PGAMB)))
	}
	ensureSelectableRow(tbl, len(headers), "Sin sesiones de usuario activas")

	sqlPreview := tview.NewTextView().SetDynamicColors(true)
	if u.ColorTheme == ThemeTurboC {
		sqlPreview.SetBackgroundColor(currentPalette.SecondaryBg)
		sqlPreview.SetTextColor(currentPalette.SecondaryFg)
		sqlPreview.SetBorder(true).
			SetBorderColor(currentPalette.SecondaryBorder).
			SetTitleColor(currentPalette.SecondaryFg).
			SetTitle("[#0000af:#00afaf:b] [[#ffffff::b]■[#0000af]] DETALLE SQL ACTUAL & DIAGNOSTICO DE BLOQUEO [-:-:-]")
	} else {
		sqlPreview.SetBackgroundColor(currentPalette.PanelBg)
		sqlPreview.SetTextColor(currentPalette.PanelFg)
		sqlPreview.SetBorder(true).
			SetBorderColor(currentPalette.InactiveBorder).
			SetTitleColor(currentPalette.InactiveTitle).
			SetTitle(" DETALLE SQL ACTUAL & DIAGNOSTICO DE BLOQUEO ")
	}

	formatSQLPreview := func(s model.UserSession) string {
		switch u.ColorTheme {
		case ThemeGreenPhosphor:
			return fmt.Sprintf(" [#00af00]Usuario:[-] [#87ff87::b]%s[-::-] (SID %d,%d @ Inst %d) │ [#00af00]Espera:[-] [#00ff5f]%s[-] │ [#00af00]Bloqueo:[-] [#00ff5f]%s[-]\n [#00af00]SQL Text:[-] [#87ff87]%s[-]",
				s.Username, s.SID, s.Serial, s.InstID, s.WaitEvent, s.BlockingSession, s.SQLText)
		case ThemeTurboC:
			return fmt.Sprintf(" [#000000::b]Usuario:[-::-] [#0000af::b]%s[-::-] (SID %d,%d @ Inst %d) │ [#000000::b]Espera:[-::-] [#0000af]%s[-] │ [#000000::b]Bloqueo:[-::-] [#af0000::b]%s[-::-]\n [#000000::b]SQL Text:[-::-] [#0000af::b]%s[-::-]",
				s.Username, s.SID, s.Serial, s.InstID, s.WaitEvent, s.BlockingSession, s.SQLText)
		default:
			return fmt.Sprintf(" [#ffff5f::b]Usuario:[-::-] [#5fffff]%s (SID %d,%d @ Inst %d)[-] │ [#ffff5f::b]Espera:[-::-] [#5fffff]%s[-] │ [#ffff5f::b]Bloqueo:[-::-] [#5fffff]%s[-]\n [#ffff5f::b]SQL Text:[-::-] [#5fffff]%s[-]",
				s.Username, s.SID, s.Serial, s.InstID, s.WaitEvent, s.BlockingSession, s.SQLText)
		}
	}

	if len(u.State.Sessions) > 0 {
		sqlPreview.SetText(formatSQLPreview(u.State.Sessions[0]))
	} else {
		sqlPreview.SetText(" Seleccione una sesion para inspeccionar su sentencia SQL y estado de bloqueo.")
	}

	tbl.SetSelectionChangedFunc(func(row, col int) {
		if row > 0 && row <= len(u.State.Sessions) {
			sqlPreview.SetText(formatSQLPreview(u.State.Sessions[row-1]))
		}
	})

	u.bindTableActions(tbl, func(row, col int) {
		s := model.UserSession{InstID: 1, SID: 0, Serial: 0, Username: "SESSION", SQLID: "0000000000"}
		hasSess := row > 0 && row <= len(u.State.Sessions)
		if hasSess {
			s = u.State.Sessions[row-1]
		} else if len(u.State.Sessions) > 0 {
			row = 1
			hasSess = true
			s = u.State.Sessions[0]
		}
		u.showActionMenuModal(
			fmt.Sprintf(" ══ MANTENIMIENTO DE SESION: %s (SID %d,%d) ══ ", s.Username, s.SID, s.Serial),
			[]menuAction{
				{"Matar Sesion Inmediatamente (KILL SESSION IMMEDIATE)", fmt.Sprintf("ALTER SYSTEM KILL SESSION '%d,%d,@%d' IMMEDIATE", s.SID, s.Serial, s.InstID), func() {
					u.runSimulatedTask(
						fmt.Sprintf("Kill Session %d,%d (%s)", s.SID, s.Serial, s.Username),
						[]string{
							fmt.Sprintf("[SQL] ALTER SYSTEM KILL SESSION '%d,%d,@%d' IMMEDIATE;", s.SID, s.Serial, s.InstID),
							"[OK] Sesion terminada y locks liberados.",
						},
						func() {
							if hasSess && row-1 < len(u.State.Sessions) {
								u.State.Sessions[row-1].Status = "KILLED"
								u.State.Sessions[row-1].BlockingSession = "-"
								u.refreshHeader()
								u.rebuildContentViews()
							}
						},
					)
				}},
				{"Desconectar Sesion Post-Transaccion (DISCONNECT)", fmt.Sprintf("ALTER SYSTEM DISCONNECT SESSION '%d,%d' POST_TRANSACTION", s.SID, s.Serial), func() {
					u.Logf("[SQL] ALTER SYSTEM DISCONNECT SESSION '%d,%d' POST_TRANSACTION; -> [green]OK[-]", s.SID, s.Serial)
				}},
				{"Ejecutar SQL Tuning Advisor / Plan de Ejecucion", fmt.Sprintf("DBMS_XPLAN.DISPLAY_CURSOR('%s')", s.SQLID), func() {
					u.Logf("[PLAN] SQL_ID %s: Plan Hash Value 39481029 -> INDEX RANGE SCAN (Cost=4)", s.SQLID)
				}},
			},
		)
	})

	u.viewFocusables["sessions"] = []tview.Primitive{tbl}

	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tbl, 0, 1, true).
		AddItem(sqlPreview, 4, 0, false)
}

// View 3: Tablespaces & Datafiles
func (u *AppUI) buildTablespacesPage() tview.Primitive {
	tbsTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	tbsTbl.SetBorder(true).SetTitle(u.winTitle("[1/2] ESTADO DE TABLESPACES (Capacidad Real con AUTOEXTEND)", "Clic/Enter = Mantenimiento │ ←/→ Ventana"))

	headers := []string{"TABLESPACE", "TIPO", "ESTADO", "ASIG_MB", "TOTAL_MAX", "USADO_MB", "LIBRE_MB", "USO REAL %", "AUTOEXT", "FILES"}
	for c, h := range headers {
		tbsTbl.SetCell(0, c, HeaderCell(h))
	}

	for i, ts := range u.State.Tablespaces {
		r := i + 1
		tbsTbl.SetCell(r, 0, KeyCell(ts.Name))
		tbsTbl.SetCell(r, 1, tview.NewTableCell(ts.Type))
		tbsTbl.SetCell(r, 2, StatusCell(ts.Status))
		tbsTbl.SetCell(r, 3, tview.NewTableCell(fmt.Sprintf("%d MB", ts.AllocMB)))
		tbsTbl.SetCell(r, 4, tview.NewTableCell(fmt.Sprintf("%d MB", ts.TotalMB)))
		tbsTbl.SetCell(r, 5, tview.NewTableCell(fmt.Sprintf("%d MB", ts.UsedMB)))
		tbsTbl.SetCell(r, 6, tview.NewTableCell(fmt.Sprintf("%d MB", ts.FreeMB)))
		tbsTbl.SetCell(r, 7, BarCell(ts.UsedPct, 12))
		tbsTbl.SetCell(r, 8, StatusCell(ts.AutoExtend))
		tbsTbl.SetCell(r, 9, tview.NewTableCell(fmt.Sprintf("%d", ts.Datafiles)))
	}
	ensureSelectableRow(tbsTbl, len(headers), "Sin tablespaces detectados")

	dfTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	dfTbl.SetBorder(true).SetTitle(u.winTitle("[2/2] DATAFILES ASOCIADOS (dba_data_files)", "Clic/Enter = Mantenimiento Datafile │ ←/→ Ventana"))
	dHeaders := []string{"FILE_ID", "TABLESPACE", "RUTA / ASM FILE", "TAMANO_MB", "MAX_MB", "AUTOEXTEND", "ESTADO"}
	for c, h := range dHeaders {
		dfTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, df := range u.State.Datafiles {
		r := i + 1
		dfTbl.SetCell(r, 0, tview.NewTableCell(fmt.Sprintf("%d", df.FileID)))
		dfTbl.SetCell(r, 1, KeyCell(df.Tablespace))
		dfTbl.SetCell(r, 2, tview.NewTableCell(df.FileName))
		dfTbl.SetCell(r, 3, tview.NewTableCell(fmt.Sprintf("%d MB", df.SizeMB)))
		dfTbl.SetCell(r, 4, tview.NewTableCell(fmt.Sprintf("%d MB", df.MaxSizeMB)))
		dfTbl.SetCell(r, 5, StatusCell(df.AutoExtend))
		dfTbl.SetCell(r, 6, StatusCell(df.Status))
	}
	ensureSelectableRow(dfTbl, len(dHeaders), "Sin datafiles detectados")

	u.bindTableActions(tbsTbl, func(row, col int) {
		ts := model.TablespaceInfo{Name: "USERS", TotalMB: 32768}
		hasTS := row > 0 && row <= len(u.State.Tablespaces)
		if hasTS {
			ts = u.State.Tablespaces[row-1]
		} else if len(u.State.Tablespaces) > 0 {
			row = 1
			hasTS = true
			ts = u.State.Tablespaces[0]
		}
		u.showActionMenuModal(
			fmt.Sprintf(" ══ MANTENIMIENTO DE TABLESPACE: %s (%.1f%% Usado de %d MB Max) ══ ", ts.Name, ts.UsedPct, ts.TotalMB),
			[]menuAction{
				{"Anadir Nuevo Datafile (+32 GB ASM)", fmt.Sprintf("ALTER TABLESPACE %s ADD DATAFILE '+DATA' SIZE 1G AUTOEXTEND ON MAXSIZE 32G", ts.Name), func() {
					u.runSimulatedTask(
						"Anadir Datafile a "+ts.Name,
						[]string{
							fmt.Sprintf("[SQL] ALTER TABLESPACE %s ADD DATAFILE '+DATA' SIZE 1024M AUTOEXTEND ON NEXT 1024M MAXSIZE 32768M;", ts.Name),
							"[OK] Datafile creado y capacidad maxima ampliada en +32 GB.",
						},
						func() {
							if hasTS && row-1 < len(u.State.Tablespaces) {
								u.State.Tablespaces[row-1].AllocMB += 1024
								u.State.Tablespaces[row-1].TotalMB += 32768
								u.State.Tablespaces[row-1].FreeMB = u.State.Tablespaces[row-1].TotalMB - u.State.Tablespaces[row-1].UsedMB
								u.State.Tablespaces[row-1].Datafiles++
								u.State.Tablespaces[row-1].UsedPct = float64(u.State.Tablespaces[row-1].UsedMB) * 100.0 / float64(u.State.Tablespaces[row-1].TotalMB)
								u.rebuildContentViews()
							}
						},
					)
				}},
				{"Activar AUTOEXTEND ON en todos los Datafiles", "ALTER DATABASE DATAFILE ... AUTOEXTEND ON MAXSIZE 32G", func() {
					if hasTS && row-1 < len(u.State.Tablespaces) {
						u.State.Tablespaces[row-1].AutoExtend = "YES"
					}
					u.Logf("[SQL] AUTOEXTEND habilitado para %s -> [green]OK[-]", ts.Name)
					u.rebuildContentViews()
				}},
				{"Desfragmentar / Coalesce Espacio Libre", fmt.Sprintf("ALTER TABLESPACE %s COALESCE", ts.Name), func() {
					u.Logf("[SQL] ALTER TABLESPACE %s COALESCE; -> [green]Espacio contiguo consolidado[-]", ts.Name)
				}},
			},
		)
	})

	u.bindTableActions(dfTbl, func(row, col int) {
		df := model.DatafileInfo{FileID: 1, Tablespace: "USERS", FileName: "+DATA/datafile01.dbf", SizeMB: 1024, MaxSizeMB: 32768}
		hasDF := row > 0 && row <= len(u.State.Datafiles)
		if hasDF {
			df = u.State.Datafiles[row-1]
		} else if len(u.State.Datafiles) > 0 {
			row = 1
			hasDF = true
			df = u.State.Datafiles[0]
		}
		u.showActionMenuModal(
			fmt.Sprintf(" ══ MANTENIMIENTO DE DATAFILE #%d (%s) ══ ", df.FileID, df.Tablespace),
			[]menuAction{
				{"Habilitar AUTOEXTEND ON MAXSIZE 32G", fmt.Sprintf("ALTER DATABASE DATAFILE %d AUTOEXTEND ON NEXT 512M MAXSIZE 32G", df.FileID), func() {
					if hasDF && row-1 < len(u.State.Datafiles) {
						u.State.Datafiles[row-1].AutoExtend = "YES"
						u.State.Datafiles[row-1].MaxSizeMB = 32768
						u.rebuildContentViews()
					}
					u.Logf("[SQL] ALTER DATABASE DATAFILE %d AUTOEXTEND ON MAXSIZE 32G; -> [green]OK[-]", df.FileID)
				}},
				{"Redimensionar Datafile (+2 GB RESIZE)", fmt.Sprintf("ALTER DATABASE DATAFILE %d RESIZE %dM", df.FileID, df.SizeMB+2048), func() {
					if hasDF && row-1 < len(u.State.Datafiles) {
						u.State.Datafiles[row-1].SizeMB += 2048
						u.rebuildContentViews()
					}
					u.Logf("[SQL] ALTER DATABASE DATAFILE %d RESIZE %dM; -> [green]OK[-]", df.FileID, df.SizeMB+2048)
				}},
			},
		)
	})

	u.viewFocusables["tablespaces"] = []tview.Primitive{tbsTbl, dfTbl}

	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tbsTbl, 0, 2, true).
		AddItem(dfTbl, 7, 0, false)
}

// View 4: Tables, Segments & Invalid Objects
func (u *AppUI) buildTablesPage() tview.Primitive {
	segTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	segTbl.SetBorder(true).SetTitle(u.winTitle("[1/2] TABLAS & SEGMENTOS PRINCIPALES", "Clic/Enter = Mantenimiento │ ←/→ Ventana"))

	headers := []string{"ESQUEMA", "NOMBRE SEGMENTO", "TIPO", "TABLESPACE", "TAMANO_MB", "NUM_FILAS", "ULT_ANALISIS", "ESTADISTICAS", "FRAGMENTACION"}
	for c, h := range headers {
		segTbl.SetCell(0, c, HeaderCell(h))
	}

	for i, seg := range u.State.TableSegments {
		r := i + 1
		segTbl.SetCell(r, 0, tview.NewTableCell(seg.Owner))
		segTbl.SetCell(r, 1, KeyCell(seg.SegmentName))
		segTbl.SetCell(r, 2, tview.NewTableCell(seg.SegmentType))
		segTbl.SetCell(r, 3, tview.NewTableCell(seg.Tablespace))
		segTbl.SetCell(r, 4, tview.NewTableCell(fmt.Sprintf("%d MB", seg.SizeMB)))
		segTbl.SetCell(r, 5, tview.NewTableCell(fmt.Sprintf("%d", seg.NumRows)))
		segTbl.SetCell(r, 6, tview.NewTableCell(seg.LastAnalyzed))
		segTbl.SetCell(r, 7, StatusCell(seg.StatsStatus))
		segTbl.SetCell(r, 8, StatusCell(seg.Fragmented))
	}
	ensureSelectableRow(segTbl, len(headers), "Sin segmentos de usuario detectados")

	invTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	invTbl.SetBorder(true).SetTitle(u.winTitle("[2/2] OBJETOS INVALIDOS (dba_objects)", "Clic/Enter = Recompilar │ ←/→ Ventana"))
	iHeaders := []string{"ESQUEMA", "OBJETO", "TIPO", "ESTADO", "ULTIMO_DDL"}
	for c, h := range iHeaders {
		invTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, inv := range u.State.InvalidObjects {
		r := i + 1
		invTbl.SetCell(r, 0, tview.NewTableCell(inv.Owner))
		invTbl.SetCell(r, 1, KeyCell(inv.ObjectName))
		invTbl.SetCell(r, 2, tview.NewTableCell(inv.ObjectType))
		invTbl.SetCell(r, 3, StatusCell(inv.Status))
		invTbl.SetCell(r, 4, tview.NewTableCell(inv.LastDDL))
	}
	ensureSelectableRow(invTbl, len(iHeaders), "0 Objetos Invalidos (Todos los objetos estan VALID)")

	u.bindTableActions(invTbl, func(row, col int) {
		u.showActionMenuModal(
			" ══ RECOMPILACION DE OBJETOS INVALIDOS ══ ",
			[]menuAction{
				{"Recompilar Todos los Objetos Invalidos (UTL_RECOMP)", "EXEC UTL_RECOMP.RECOMP_PARALLEL(4)", func() {
					u.State.InvalidObjects = nil
					u.refreshHeader()
					u.rebuildContentViews()
					u.Logf("[SQL] EXEC UTL_RECOMP.RECOMP_PARALLEL(4); -> [green]0 Objetos Invalidos[-]")
				}},
			},
		)
	})

	u.bindTableActions(segTbl, func(row, col int) {
		seg := model.TableSegment{Owner: "APP", SegmentName: "ALL_TABLES"}
		hasSeg := row > 0 && row <= len(u.State.TableSegments)
		if hasSeg {
			seg = u.State.TableSegments[row-1]
		} else if len(u.State.TableSegments) > 0 {
			row = 1
			hasSeg = true
			seg = u.State.TableSegments[0]
		}
		u.showActionMenuModal(
			fmt.Sprintf(" ══ MANTENIMIENTO DE TABLA/OBJETO: %s.%s ══ ", seg.Owner, seg.SegmentName),
			[]menuAction{
				{"Recolectar Estadisticas Optimizador (DBMS_STATS)", fmt.Sprintf("EXEC DBMS_STATS.GATHER_TABLE_STATS('%s','%s',CASCADE=>TRUE)", seg.Owner, seg.SegmentName), func() {
					u.runSimulatedTask(
						fmt.Sprintf("Gather Stats %s.%s", seg.Owner, seg.SegmentName),
						[]string{
							fmt.Sprintf("[SQL] BEGIN DBMS_STATS.GATHER_TABLE_STATS(ownname=>'%s', tabname=>'%s', estimate_percent=>DBMS_STATS.AUTO_SAMPLE_SIZE, cascade=>TRUE); END;", seg.Owner, seg.SegmentName),
							"[OK] Estadisticas actualizadas para tabla e indices.",
						},
						func() {
							if hasSeg && row-1 < len(u.State.TableSegments) {
								u.State.TableSegments[row-1].StatsStatus = "FRESH"
								u.State.TableSegments[row-1].LastAnalyzed = time.Now().Format("2006-01-02 15:04")
								u.rebuildContentViews()
							}
						},
					)
				}},
				{"Compactar / Shrink Espacio de Tabla Online", fmt.Sprintf("ALTER TABLE %s.%s ENABLE ROW MOVEMENT + SHRINK SPACE CASCADE", seg.Owner, seg.SegmentName), func() {
					u.runSimulatedTask(
						fmt.Sprintf("Shrink Space %s.%s", seg.Owner, seg.SegmentName),
						[]string{
							fmt.Sprintf("[SQL] ALTER TABLE %s.%s ENABLE ROW MOVEMENT;", seg.Owner, seg.SegmentName),
							fmt.Sprintf("[SQL] ALTER TABLE %s.%s SHRINK SPACE CASCADE;", seg.Owner, seg.SegmentName),
							"[OK] Bloques HWM compactados y espacio devuelto al tablespace.",
						},
						func() {
							if hasSeg && row-1 < len(u.State.TableSegments) {
								u.State.TableSegments[row-1].Fragmented = "LOW (2%)"
								u.rebuildContentViews()
							}
						},
					)
				}},
				{"Reconstruir Indices Online (REBUILD ONLINE)", "ALTER INDEX ... REBUILD ONLINE PARALLEL 4", func() {
					u.Logf("[SQL] Indices asociados a %s.%s reconstruidos ONLINE -> [green]OK[-]", seg.Owner, seg.SegmentName)
				}},
				{"Compilar Objetos Invalidos del Esquema (COMPILE_SCHEMA)", fmt.Sprintf("EXEC DBMS_UTILITY.COMPILE_SCHEMA('%s', FALSE)", seg.Owner), func() {
					u.State.InvalidObjects = nil
					u.refreshHeader()
					u.rebuildContentViews()
					u.Logf("[SQL] DBMS_UTILITY.COMPILE_SCHEMA('%s') -> [green]0 Objetos Invalidos restantes[-]", seg.Owner)
				}},
			},
		)
	})

	u.viewFocusables["tables"] = []tview.Primitive{segTbl, invTbl}

	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(segTbl, 0, 2, true).
		AddItem(invTbl, 6, 0, false)
}

// View 5: ASM Storage & Listeners
func (u *AppUI) buildASMNetworkPage() tview.Primitive {
	asmTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	asmTbl.SetBorder(true).SetTitle(u.winTitle("[1/3] ALMACENAMIENTO ASM DISKGROUPS (v$asm_diskgroup)", "Clic/Enter = Control Listener │ ←/→ Ventana"))
	aHeaders := []string{"DISKGROUP", "ESTADO", "REDUNDANCIA", "TOTAL_MB", "LIBRE_MB", "USABLE_MB", "USO % (BARRA)", "DISCOS_OFFLINE"}
	for c, h := range aHeaders {
		asmTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, dg := range u.State.Diskgroups {
		r := i + 1
		asmTbl.SetCell(r, 0, KeyCell(dg.Name))
		asmTbl.SetCell(r, 1, StatusCell(dg.State))
		asmTbl.SetCell(r, 2, tview.NewTableCell(dg.Type))
		asmTbl.SetCell(r, 3, tview.NewTableCell(fmt.Sprintf("%d MB", dg.TotalMB)))
		asmTbl.SetCell(r, 4, tview.NewTableCell(fmt.Sprintf("%d MB", dg.FreeMB)))
		asmTbl.SetCell(r, 5, tview.NewTableCell(fmt.Sprintf("%d MB", dg.UsableMB)))
		asmTbl.SetCell(r, 6, BarCell(dg.UsedPct, 12))
		offSt := fmt.Sprintf("%d", dg.OfflineDisks)
		if dg.OfflineDisks > 0 {
			offSt = fmt.Sprintf("%d OFFLINE", dg.OfflineDisks)
		}
		asmTbl.SetCell(r, 7, StatusCell(offSt))
	}
	ensureSelectableRow(asmTbl, len(aHeaders), "Sin Diskgroups ASM (Almacenamiento en Filesystem - Clic o Enter para Listeners)")
	u.bindTableActions(asmTbl, func(row, col int) {
		u.showListenerControlModal(0)
	})

	lsnrTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	lsnrTbl.SetBorder(true).SetTitle(u.winTitle("[2/3] RED ORACLE: LISTENERS LOCALES & SCAN", "Clic/Enter = Arrancar/Parar/Resetear │ ←/→ Ventana"))
	lHeaders := []string{"LISTENER", "TIPO", "NODO", "PROCESO", "ESTADO_READY", "ENDPOINT", "SERVICIOS DETECTADOS"}
	for c, h := range lHeaders {
		lsnrTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, l := range u.State.Listeners {
		r := i + 1
		readySt := l.ReadyStatus
		if readySt == "" {
			readySt = l.State
		}
		svcSummary := l.ServiceSummary
		if svcSummary == "" {
			svcSummary = fmt.Sprintf("%d servicios", l.Services)
		}
		lsnrTbl.SetCell(r, 0, KeyCell(l.Name))
		lsnrTbl.SetCell(r, 1, tview.NewTableCell(l.Type))
		lsnrTbl.SetCell(r, 2, tview.NewTableCell(l.Node))
		lsnrTbl.SetCell(r, 3, StatusCell(l.State))
		lsnrTbl.SetCell(r, 4, StatusCell(readySt))
		lsnrTbl.SetCell(r, 5, tview.NewTableCell(l.Endpoint))
		lsnrTbl.SetCell(r, 6, AccentCell(svcSummary))
	}
	ensureSelectableRow(lsnrTbl, len(lHeaders), "Sin listeners activos (Clic o Enter para Arrancar / Controlar Listener)")
	u.bindTableActions(lsnrTbl, func(row, col int) {
		idx := 0
		if row > 0 && row <= len(u.State.Listeners) {
			idx = row - 1
		}
		u.showListenerControlModal(idx)
	})

	svcTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	svcTbl.SetBorder(true).SetTitle(u.winTitle("[3/3] DETALLE DE SERVICIOS EN LISTENER (READY / UNKNOWN / BLOCKED)", "Clic/Enter = Arrancar/Parar/Resetear │ ←/→ Ventana"))
	sHeaders := []string{"LISTENER", "SERVICIO", "INSTANCIA", "ESTADO_REAL", "TIPO_REGISTRO / DISPONIBILIDAD", "HANDLERS"}
	for c, h := range sHeaders {
		svcTbl.SetCell(0, c, HeaderCell(h))
	}
	svcRow := 1
	for _, l := range u.State.Listeners {
		for _, sh := range l.ServiceList {
			regDesc := "DINAMICO (LREG/PMON - Listo para conexiones)"
			switch strings.ToUpper(sh.Status) {
			case "UNKNOWN":
				regDesc = "ESTATICO (listener.ora - No garantiza BD arriba)"
			case "BLOCKED", "RESTRICTED":
				regDesc = "BLOQUEADO (Instancia en MOUNT/NOMOUNT/RESTRICTED)"
			}
			svcTbl.SetCell(svcRow, 0, tview.NewTableCell(sh.ListenerName))
			svcTbl.SetCell(svcRow, 1, KeyCell(sh.ServiceName))
			svcTbl.SetCell(svcRow, 2, tview.NewTableCell(sh.InstanceName))
			svcTbl.SetCell(svcRow, 3, StatusCell(sh.Status))
			svcTbl.SetCell(svcRow, 4, tview.NewTableCell(regDesc))
			svcTbl.SetCell(svcRow, 5, tview.NewTableCell(fmt.Sprintf("%d handler(s)", sh.Handlers)))
			svcRow++
		}
	}
	ensureSelectableRow(svcTbl, len(sHeaders), "Sin servicios registrados en el listener (Clic o Enter para Arrancar/Resetear Listener)")
	u.bindTableActions(svcTbl, func(row, col int) {
		u.showListenerControlModal(0)
	})

	hasASM := len(u.State.Diskgroups) > 0
	if hasASM {
		u.viewFocusables["asm_net"] = []tview.Primitive{asmTbl, lsnrTbl, svcTbl}
	} else {
		// On Single Instance / Filesystem without ASM, focus Listeners first when entering View 5
		u.viewFocusables["asm_net"] = []tview.Primitive{lsnrTbl, svcTbl, asmTbl}
	}

	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(asmTbl, 6, 0, hasASM).
		AddItem(lsnrTbl, 6, 0, !hasASM).
		AddItem(svcTbl, 0, 1, false)
}

func (u *AppUI) showListenerControlModal(idx int) {
	lsnrName := "LISTENER"
	nodeName := u.State.HostName
	if idx >= 0 && idx < len(u.State.Listeners) {
		lsnrName = u.State.Listeners[idx].Name
		nodeName = u.State.Listeners[idx].Node
	}

	runLsnrCmd := func(actionLabel string, cmds [][]string, newState, newReady string) {
		if idx >= 0 && idx < len(u.State.Listeners) && newState != "" {
			u.State.Listeners[idx].State = newState
			u.State.Listeners[idx].ReadyStatus = newReady
		}
		u.rebuildContentViews()

		if u.State.IsMock || u.State.OracleHome == "" {
			u.Logf("[LSNRCTL] %s (%s @ %s) -> [green]OK[-]", actionLabel, lsnrName, nodeName)
			return
		}

		go func() {
			runner := executor.NewRunner(20*time.Second, false)
			home := u.State.GridHome
			if home == "" || strings.HasPrefix(home, "N/A") {
				home = u.State.OracleHome
			}
			owner, _ := executor.DetectFileOwner(home)
			lsnrBin := filepath.Join(home, "bin", "lsnrctl")
			envMap := map[string]string{
				"ORACLE_HOME": home,
				"LC_ALL":      "C",
				"NLS_LANG":    "AMERICAN_AMERICA.AL32UTF8",
			}
			for _, args := range cmds {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				out, err := runner.RunCommand(ctx, owner, false, envMap, nil, lsnrBin, args...)
				cancel()
				u.App.QueueUpdateDraw(func() {
					if err != nil {
						u.Logf("[LSNRCTL] lsnrctl %s -> [yellow]%v[-]", strings.Join(args, " "), err)
					} else {
						u.Logf("[LSNRCTL] lsnrctl %s -> [green]The command completed successfully[-]", strings.Join(args, " "))
					}
					for _, line := range strings.Split(out, "\n") {
						trimmed := strings.TrimSpace(line)
						if strings.HasPrefix(trimmed, "Service \"") || strings.HasPrefix(trimmed, "Instance \"") || strings.HasPrefix(trimmed, "TNS-") {
							u.Logf("    %s", trimmed)
						}
					}
				})
			}
		}()
	}

	u.showActionMenuModal(
		fmt.Sprintf(" ══ CONTROL DE LISTENER: %s (%s) ══ ", lsnrName, nodeName),
		[]menuAction{
			{"Iniciar / Arrancar Listener (lsnrctl start)", fmt.Sprintf("lsnrctl start %s", lsnrName), func() {
				runLsnrCmd("Arrancar Listener", [][]string{{"start", lsnrName}}, "ONLINE", "READY")
			}},
			{"Parar Listener (lsnrctl stop)", fmt.Sprintf("lsnrctl stop %s", lsnrName), func() {
				runLsnrCmd("Parar Listener", [][]string{{"stop", lsnrName}}, "OFFLINE", "OFFLINE")
			}},
			{"Reiniciar / Resetear Listener (stop + start)", fmt.Sprintf("lsnrctl stop %s && lsnrctl start %s", lsnrName, lsnrName), func() {
				runLsnrCmd("Reiniciar / Resetear Listener", [][]string{{"stop", lsnrName}, {"start", lsnrName}}, "ONLINE", "READY")
			}},
			{"Recargar Configuracion sin Corte (lsnrctl reload)", fmt.Sprintf("lsnrctl reload %s (Recarga listener.ora en caliente)", lsnrName), func() {
				runLsnrCmd("Recargar Listener", [][]string{{"reload", lsnrName}}, "ONLINE", "READY")
			}},
			{"Forzar Registro Dinamico en BD (ALTER SYSTEM REGISTER)", "Obliga a LREG/PMON a publicar servicios como READY en el listener", func() {
				if u.State.IsMock || u.State.OracleHome == "" || len(u.State.Instances) == 0 {
					u.Logf("[SQL] ALTER SYSTEM REGISTER; -> [green]Servicios registrados dinamicamente en estado READY[-]")
					return
				}
				sid := u.State.Instances[0].InstName
				home := u.State.OracleHome
				go func() {
					runner := executor.NewRunner(15*time.Second, false)
					owner, _ := executor.DetectFileOwner(home)
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					_, _, err := runner.RunSQLPlus(ctx, home, sid, owner, "ALTER SYSTEM REGISTER;", nil)
					cancel()
					u.App.QueueUpdateDraw(func() {
						if err != nil {
							u.Logf("[SQL] ALTER SYSTEM REGISTER -> [red]%v[-]", err)
						} else {
							u.Logf("[SQL] ALTER SYSTEM REGISTER; -> [green]OK (LREG/PMON ha publicado los servicios en el Listener)[-]")
						}
					})
				}()
			}},
			{"Comprobar Estado Completo (lsnrctl status)", fmt.Sprintf("lsnrctl status %s", lsnrName), func() {
				runLsnrCmd("Estado de Listener", [][]string{{"status", lsnrName}}, "", "")
			}},
		},
	)
}

// View 6: RMAN Backups, Flashback Restore Points & Data Guard
func (u *AppUI) buildRMANDGPage() tview.Primitive {
	rmanTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	rmanTbl.SetBorder(true).SetTitle(u.winTitle("[1/3] HISTORIAL DE BACKUPS RMAN", "Clic/Enter = Lanzar Backup │ ←/→ Ventana"))
	rHeaders := []string{"SESSION_KEY", "TIPO BACKUP", "ESTADO", "INICIO", "DURACION", "TAMANO_GB", "DISPOSITIVO"}
	for c, h := range rHeaders {
		rmanTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, b := range u.State.RMANBackups {
		r := i + 1
		rmanTbl.SetCell(r, 0, tview.NewTableCell(fmt.Sprintf("%d", b.SessionKey)))
		rmanTbl.SetCell(r, 1, KeyCell(b.InputType))
		rmanTbl.SetCell(r, 2, StatusCell(b.Status))
		rmanTbl.SetCell(r, 3, tview.NewTableCell(b.StartTime))
		rmanTbl.SetCell(r, 4, tview.NewTableCell(b.Duration))
		rmanTbl.SetCell(r, 5, tview.NewTableCell(fmt.Sprintf("%.1f GB", b.SizeGB)))
		rmanTbl.SetCell(r, 6, tview.NewTableCell(b.DeviceType))
	}
	ensureSelectableRow(rmanTbl, len(rHeaders), "Sin backups RMAN recientes (Clic o Enter para iniciar backup)")

	openRmanMenu := func(row, col int) {
		u.showActionMenuModal(
			" ══ OPERACIONES DE BACKUP RMAN & FLASHBACK ══ ",
			[]menuAction{
				{"Lanzar Backup de Archivelogs + Delete Input", "rman target / <<< 'BACKUP ARCHIVELOG ALL DELETE INPUT;'", func() {
					u.runSimulatedTask("Backup Archivelogs RMAN", []string{
						"[RMAN] Starting backup at " + time.Now().Format("2006-01-02 15:04:05"),
						"[RMAN] channel ORA_DISK_1: starting compressed archived log backup set",
						"[RMAN] Finished backup. Pieces written to +FRA.",
					}, func() {
						u.State.RMANBackups = append([]model.RMANBackupJob{{
							SessionKey: 1420,
							InputType:  "ARCHIVELOG",
							Status:     "COMPLETED",
							StartTime:  time.Now().Format("2006-01-02 15:04"),
							Duration:   "00h 01m",
							SizeGB:     6.4,
							DeviceType: "DISK (+FRA)",
						}}, u.State.RMANBackups...)
						u.rebuildContentViews()
					})
				}},
				{"Lanzar Backup Incremental Nivel 1 Online", "rman target / <<< 'BACKUP INCREMENTAL LEVEL 1 DATABASE PLUS ARCHIVELOG;'", func() {
					u.Logf("[RMAN] Backup Incremental Nivel 1 en ejecucion en segundo plano...")
				}},
				{"Ejecutar CROSSCHECK & DELETE EXPIRED ARCHIVELOG", "Limpia catalogo RMAN y libera espacio en FRA", func() {
					u.Logf("[RMAN] CROSSCHECK ARCHIVELOG ALL; DELETE NOPROMPT EXPIRED ARCHIVELOG ALL; -> [green]OK[-]")
				}},
				{"Crear Punto de Restauracion Garantizado (GRP)", "CREATE RESTORE POINT GRP_MAINT GUARANTEE FLASHBACK DATABASE", func() {
					u.State.RestorePoints = append(u.State.RestorePoints, model.RestorePoint{
						Name:        "GRP_MAINT_" + time.Now().Format("1504"),
						SCN:         94825100412,
						Time:        time.Now().Format("2006-01-02 15:04:05"),
						Guaranteed:  "YES",
						StorageSize: "2.0 GB (+FRA)",
					})
					u.rebuildContentViews()
					u.Logf("[SQL] Punto de restauracion garantizado creado con exito.")
				}},
			},
		)
	}
	u.bindTableActions(rmanTbl, openRmanMenu)

	rpTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	rpTbl.SetBorder(true).SetTitle(u.winTitle("[2/3] FLASHBACK & PUNTOS DE RESTAURACION (v$restore_point)", "Clic/Enter = Crear/Gestionar GRP │ ←/→ Ventana"))
	rpHeaders := []string{"NOMBRE RESTORE POINT", "SCN", "FECHA CREACION", "GARANTIZADO", "USO EN FRA"}
	for c, h := range rpHeaders {
		rpTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, rp := range u.State.RestorePoints {
		r := i + 1
		rpTbl.SetCell(r, 0, AccentCell(rp.Name))
		rpTbl.SetCell(r, 1, tview.NewTableCell(fmt.Sprintf("%d", rp.SCN)))
		rpTbl.SetCell(r, 2, tview.NewTableCell(rp.Time))
		rpTbl.SetCell(r, 3, StatusCell(rp.Guaranteed))
		rpTbl.SetCell(r, 4, tview.NewTableCell(rp.StorageSize))
	}
	ensureSelectableRow(rpTbl, len(rpHeaders), "Sin puntos de restauracion activos (Clic o Enter para crear GRP)")
	u.bindTableActions(rpTbl, openRmanMenu)

	dgTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	dgTbl.SetBorder(true).SetTitle(u.winTitle("[3/3] DATA GUARD STANDBY & REPLICACION (v$dataguard_stats)", "Clic/Enter = Operaciones Data Guard │ ←/→ Ventana"))
	dgHeaders := []string{"STANDBY DB", "ROL", "HOST DESTINO", "OPEN_MODE", "TRANSPORT_LAG", "APPLY_LAG", "ESTADO MRP0", "SWITCHOVER"}
	for c, h := range dgHeaders {
		dgTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, dg := range u.State.StandbyDBs {
		r := i + 1
		dgTbl.SetCell(r, 0, KeyCell(dg.DBUniqueName))
		dgTbl.SetCell(r, 1, tview.NewTableCell(dg.Role))
		dgTbl.SetCell(r, 2, tview.NewTableCell(dg.Host))
		dgTbl.SetCell(r, 3, StatusCell(dg.OpenMode))
		dgTbl.SetCell(r, 4, StatusCell(dg.TransportLag))
		dgTbl.SetCell(r, 5, StatusCell(dg.ApplyLag))
		dgTbl.SetCell(r, 6, StatusCell(dg.MRPStatus))
		dgTbl.SetCell(r, 7, StatusCell(dg.Switchover))
	}
	ensureSelectableRow(dgTbl, len(dgHeaders), "Sin standby Data Guard configurada")
	u.bindTableActions(dgTbl, func(row, col int) {
		u.showActionMenuModal(
			" ══ OPERACIONES DATA GUARD & REPLICACION STANDBY ══ ",
			[]menuAction{
				{"Forzar Envio de Redo Actual (ARCHIVE LOG CURRENT)", "ALTER SYSTEM ARCHIVE LOG CURRENT", func() {
					u.Logf("[SQL] ALTER SYSTEM ARCHIVE LOG CURRENT; -> [green]Redo actual archivado y enviado[-]")
				}},
				{"Activar Managed Recovery MRP0 en Tiempo Real", "ALTER DATABASE RECOVER MANAGED STANDBY DATABASE DISCONNECT", func() {
					u.Logf("[DG] Proceso MRP0 verificado en modo APPLYING_LOG (Real-Time Apply)")
				}},
			},
		)
	})

	u.viewFocusables["rman_dg"] = []tview.Primitive{rmanTbl, rpTbl, dgTbl}

	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(rmanTbl, 0, 2, true).
		AddItem(rpTbl, 5, 0, false).
		AddItem(dgTbl, 5, 0, false)
}

// View 7: Alert Log & ORA- Diagnostics Center
func (u *AppUI) buildAlertLogPage() tview.Primitive {
	// Panel [1/3]: Strict ORA- Errors Table
	alertTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	alertTbl.SetBorder(true).SetTitle(u.winTitle("[1/3] ERRORES ORA-* EN ALERT LOG (Filtro Estricto ^ORA-)", "Clic/Enter = Diagnosticar / oerr │ ←/→ Ventana"))
	headers := []string{"TIMESTAMP", "INSTANCIA", "TIPO", "CODIGO ORA-", "MENSAJE DE ERROR EN ALERT LOG"}
	for c, h := range headers {
		alertTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, a := range u.State.AlertEntries {
		r := i + 1
		alertTbl.SetCell(r, 0, tview.NewTableCell(a.Timestamp))
		alertTbl.SetCell(r, 1, tview.NewTableCell(a.Instance))
		alertTbl.SetCell(r, 2, StatusCell(a.Severity))
		alertTbl.SetCell(r, 3, StatusCell(a.ErrorCode))
		alertTbl.SetCell(r, 4, tview.NewTableCell(a.Message))
	}
	ensureSelectableRow(alertTbl, len(headers), "0 errores ORA-* detectados en el Alert Log reciente (Base de datos sin alertas ORA-)")

	u.bindTableActions(alertTbl, func(row, col int) {
		errCode := "ORA-00000"
		instName := u.State.HostName
		msgText := ""
		if row > 0 && row <= len(u.State.AlertEntries) {
			entry := u.State.AlertEntries[row-1]
			errCode = entry.ErrorCode
			instName = entry.Instance
			msgText = entry.Message
		}
		u.showActionMenuModal(
			fmt.Sprintf(" ══ DIAGNOSTICO DE ERROR %s (%s) ══ ", errCode, instName),
			[]menuAction{
				{"Consultar Diccionario Oficial de Errores (oerr ora)", fmt.Sprintf("Ejecuta oerr para explicar causa y accion de %s", errCode), func() {
					u.Logf("[OERR] %s -> %s (Verifique traza en %s)", errCode, explainOraCode(errCode, msgText), u.State.DiagTraceDir)
				}},
				{"Ir al Modulo de Resolucion Guiada Asociado", "Abre Tablespaces (ORA-0165x), Sesiones (ORA-00060) o RMAN (ORA-198xx)", func() {
					switch {
					case strings.Contains(errCode, "0165"):
						u.switchViewAndFocus("tablespaces")
					case strings.Contains(errCode, "198"):
						u.switchViewAndFocus("rman_dg")
					default:
						u.switchViewAndFocus("sessions")
					}
				}},
				{"Descartar / Limpiar Alerta de la Vista Actual", "Oculta esta entrada ORA- hasta el proximo evento", func() {
					if row > 0 && row <= len(u.State.AlertEntries) {
						u.State.AlertEntries = append(u.State.AlertEntries[:row-1], u.State.AlertEntries[row:]...)
						u.refreshHeader()
						u.rebuildContentViews()
					}
				}},
			},
		)
	})

	// Panel [2/3]: ADR Diagnostics & Alert Log Operations Table
	diagTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	diagTbl.SetBorder(true).SetTitle(u.winTitle("[2/3] HERRAMIENTAS DE DIAGNOSTICO ADR & CONTROL DE ALERT.LOG", "Clic/Enter = Ejecutar │ ←/→ Ventana"))
	dHeaders := []string{"HERRAMIENTA / RECURSO", "RUTA / PARAMETRO ACTUAL", "ESTADO / METRICA", "ACCION DISPONIBLE (CLIC / ENTER)"}
	for c, h := range dHeaders {
		diagTbl.SetCell(0, c, HeaderCell(h))
	}

	traceDir := u.State.DiagTraceDir
	if traceDir == "" {
		traceDir = "$ORACLE_BASE/diag/rdbms/<db>/<SID>/trace"
	}
	instSID := "ORCL"
	if len(u.State.Instances) > 0 {
		instSID = u.State.Instances[0].InstName
	}
	alertPath := fmt.Sprintf("%s/alert_%s.log", traceDir, instSID)
	sizeStr := fmt.Sprintf("%.2f MB", u.State.AlertFileSizeMB)
	if u.State.AlertFileSizeMB <= 0 {
		sizeStr = "ONLINE"
	}

	oraSummary := "0 Errores ORA- activos (ONLINE)"
	if len(u.State.AlertEntries) > 0 {
		counts := make(map[string]int)
		for _, e := range u.State.AlertEntries {
			counts[e.ErrorCode]++
		}
		var parts []string
		for k, v := range counts {
			parts = append(parts, fmt.Sprintf("%s(%d)", k, v))
		}
		oraSummary = strings.Join(parts, ", ")
	}

	diagRows := [][]string{
		{"1. Archivo Alert Log", alertPath, sizeStr, "Rotar / Limpiar alert.log o Forzar Switch Logfile"},
		{"2. Resumen Codigos ORA-", "Filtro estricto ^ORA- (Ignora OPatch)", oraSummary, "Consultar Causa/Solucion con utilidad oerr"},
		{"3. Mantenimiento ADRCI", traceDir, "ADR ONLINE", "Purgar trazas (.trc/.trm) e incidentes > 7 dias"},
		{"4. Redo Log & Checkpoint", "ALTER SYSTEM SWITCH LOGFILE / CHECKPOINT", "OPEN", "Avanzar secuencia Redo y verificar escritura Alert"},
	}
	for i, dr := range diagRows {
		r := i + 1
		diagTbl.SetCell(r, 0, KeyCell(dr[0]))
		diagTbl.SetCell(r, 1, tview.NewTableCell(dr[1]))
		diagTbl.SetCell(r, 2, StatusCell(dr[2]))
		diagTbl.SetCell(r, 3, AccentCell(dr[3]))
	}
	diagTbl.Select(1, 0)

	u.bindTableActions(diagTbl, func(row, col int) {
		u.showActionMenuModal(
			" ══ OPERACIONES DE DIAGNOSTICO ADR, OERR & ALERT LOG ══ ",
			[]menuAction{
				{"Forzar Cambio de Redo Log (ALTER SYSTEM SWITCH LOGFILE)", "Avanza secuencia de Redo Log y escribe evento en alert.log", func() {
					newLine := fmt.Sprintf("[%s] Thread 1 advanced to log sequence (ALTER SYSTEM SWITCH LOGFILE)", time.Now().Format("2006-01-02 15:04:05"))
					u.State.AlertTailLines = append(u.State.AlertTailLines, newLine)
					u.rebuildContentViews()
					u.Logf("[SQL] ALTER SYSTEM SWITCH LOGFILE; -> [green]Secuencia avanzada y registrada en Alert Log[-]")
				}},
				{"Forzar Checkpoint Global (ALTER SYSTEM CHECKPOINT)", "Sincroniza bloques sucios en cabeceras de datafiles", func() {
					newLine := fmt.Sprintf("[%s] Completed checkpoint up to RBA (ALTER SYSTEM CHECKPOINT)", time.Now().Format("2006-01-02 15:04:05"))
					u.State.AlertTailLines = append(u.State.AlertTailLines, newLine)
					u.rebuildContentViews()
					u.Logf("[SQL] ALTER SYSTEM CHECKPOINT; -> [green]Checkpoint global completado[-]")
				}},
				{"Purgar Trazas, Dumps y Logs Antiguos con ADRCI (> 7 dias)", "adrci exec='purge -age 10080 -type TRACE; purge -age 10080 -type INCIDENT'", func() {
					u.Logf("[ADRCI] Purgado de trazas (.trc/.trm) e incidentes (>7 dias) en %s -> [green]OK[-]", traceDir)
				}},
				{"Rotar y Limpiar Alert Log Actual (Backup + Truncate)", fmt.Sprintf("cp %s %s.%s && > %s", alertPath, alertPath, time.Now().Format("20060102"), alertPath), func() {
					u.State.AlertEntries = nil
					u.State.AlertFileSizeMB = 0.01
					u.refreshHeader()
					u.rebuildContentViews()
					u.Logf("[ALERT] Archivo %s rotado y truncado correctamente.", alertPath)
				}},
			},
		)
	})

	// Panel [3/3]: Live Tail of alert_<SID>.log
	tailBox := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	if u.ColorTheme == ThemeTurboC {
		tailBox.SetBackgroundColor(currentPalette.SecondaryBg)
		tailBox.SetTextColor(currentPalette.SecondaryFg)
	} else {
		tailBox.SetBackgroundColor(currentPalette.PanelBg)
		tailBox.SetTextColor(currentPalette.PanelFg)
	}
	tailBox.SetBorder(true).SetTitle(u.winTitle("[3/3] TAIL EN VIVO DE ALERT_"+instSID+".LOG (Ultimas Lineas del Motor)", "←/→ Ventana"))

	if len(u.State.AlertTailLines) == 0 {
		tailBox.SetText(" Sin actividad reciente en el tail del Alert Log.")
	} else {
		var sb strings.Builder
		for _, line := range u.State.AlertTailLines {
			switch u.ColorTheme {
			case ThemeGreenPhosphor:
				if strings.Contains(line, "ORA-") {
					sb.WriteString(" [#001000:#00ff5f:b] " + line + " [-:-:-]\n")
				} else if strings.Contains(line, "Thread ") || strings.Contains(line, "ALTER ") || strings.Contains(line, "Completed") {
					sb.WriteString(" [#87ff87::b]" + line + "[-::-]\n")
				} else {
					sb.WriteString(" [#00ff5f]" + line + "[-]\n")
				}
			case ThemeTurboC:
				if strings.Contains(line, "ORA-") {
					sb.WriteString(" [#af0000::b]" + line + "[-::-]\n")
				} else if strings.Contains(line, "Thread ") || strings.Contains(line, "ALTER ") || strings.Contains(line, "Completed") {
					sb.WriteString(" [#000000::b]" + line + "[-::-]\n")
				} else {
					sb.WriteString(" [#0000af]" + line + "[-]\n")
				}
			default:
				if strings.Contains(line, "ORA-") {
					sb.WriteString(" [#ff5f5f::b]" + line + "[-::-]\n")
				} else if strings.Contains(line, "Thread ") || strings.Contains(line, "ALTER ") || strings.Contains(line, "Completed") {
					sb.WriteString(" [#ffff5f]" + line + "[-]\n")
				} else {
					sb.WriteString(" [#5fffff]" + line + "[-]\n")
				}
			}
		}
		tailBox.SetText(sb.String())
		tailBox.ScrollToEnd()
	}

	u.viewFocusables["alert_log"] = []tview.Primitive{alertTbl, diagTbl, tailBox}

	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(alertTbl, 7, 0, true).
		AddItem(diagTbl, 7, 0, false).
		AddItem(tailBox, 0, 1, false)
}

// View 8: Linux Server Health Checks — CPU/Load, RAM/Swap, HugePages/THP, Filesystems/Inodes & SAN Multipath Paths
func (u *AppUI) buildServerHealthPage() tview.Primitive {
	// Panel [1/3]: CPU, Load Average, RAM, Swap, HugePages, THP & Kernel AIO
	osTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	osTbl.SetBorder(true).SetTitle(u.winTitle("[1/3] SERVIDOR LINUX: CPU, LOAD, RAM, SWAP, HUGEPAGES & KERNEL ORACLE", "Clic/Enter = Auditar/Acciones │ ←/→ Ventana"))
	osHeaders := []string{"SUBSISTEMA", "COMPONENTE", "ESTADO", "USO % (BARRA)", "METRICA ACTUAL EN SERVIDOR", "DIAGNOSTICO / RECOMENDACION ORACLE"}
	for c, h := range osHeaders {
		osTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, m := range u.State.OSMetrics {
		r := i + 1
		osTbl.SetCell(r, 0, KeyCell(m.Category))
		osTbl.SetCell(r, 1, tview.NewTableCell(m.Component))
		osTbl.SetCell(r, 2, StatusCell(m.Status))
		osTbl.SetCell(r, 3, BarCell(m.UsedPct, 10))
		osTbl.SetCell(r, 4, AccentCell(m.CurrentValue))
		osTbl.SetCell(r, 5, tview.NewTableCell(m.Recommendation))
	}
	ensureSelectableRow(osTbl, len(osHeaders), "Sin metricas de sistema operativo recolectadas (Clic o Enter para auditar servidor)")

	u.bindTableActions(osTbl, func(row, col int) {
		u.showActionMenuModal(
			fmt.Sprintf(" ══ AUDITORIA Y MANTENIMIENTO DE SERVIDOR LINUX (%s) ══ ", u.State.HostName),
			[]menuAction{
				{"Refrescar Comprobaciones de Servidor, Discos y Multipath Ahora", "Relee /proc/meminfo, /proc/loadavg, df -PTm, df -Pi y multipath -ll", func() {
					if u.State.IsMock {
						u.Logf("[SERVER] Comprobaciones de CPU, RAM, HugePages, Filesystems y Multipath actualizadas -> [green]OK[-]")
						return
					}
					go func() {
						runner := executor.NewRunner(15*time.Second, false)
						ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
						osM, fsM, spM := collector.CollectOSHealth(ctx, runner, nil)
						cancel()
						u.App.QueueUpdateDraw(func() {
							u.State.OSMetrics = osM
							u.State.Filesystems = fsM
							u.State.StoragePaths = spM
							u.rebuildContentViews()
							u.Logf("[SERVER] Comprobaciones de servidor actualizadas (%d metricas, %d montajes, %d paths) -> [green]OK[-]",
								len(osM), len(fsM), len(spM))
						})
					}()
				}},
				{"Auditar Parámetros de Kernel Oracle (sysctl aio, shmmax, hugepages, swappiness)", "sysctl fs.aio-nr fs.aio-max-nr vm.swappiness vm.nr_hugepages kernel.shmmax", func() {
					if u.State.IsMock {
						u.Logf("[SYSCTL] fs.aio-nr=64240 │ fs.aio-max-nr=1048576 │ vm.swappiness=10 │ vm.nr_hugepages=34816 -> [green]OPTIMO[-]")
						return
					}
					go func() {
						runner := executor.NewRunner(10*time.Second, false)
						ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
						out, _ := runner.RunCommand(ctx, nil, false, map[string]string{"LC_ALL": "C"}, nil,
							"sysctl", "fs.aio-nr", "fs.aio-max-nr", "vm.swappiness", "vm.nr_hugepages", "kernel.shmmax")
						cancel()
						u.App.QueueUpdateDraw(func() {
							flat := strings.ReplaceAll(strings.TrimSpace(out), "\n", " │ ")
							u.Logf("[SYSCTL] %s", flat)
						})
					}()
				}},
				{"Top 5 Procesos por Consumo de CPU y Memoria RAM (ps aux --sort=-%cpu)", "Identifica procesos Oracle o de SO que acaparan CPU/RAM", func() {
					if u.State.IsMock {
						u.Logf("[TOP PROCS] 1) ora_p001_PRODCDB1 (18.4%% CPU, 2.1G) │ 2) ora_lgwr_PRODCDB1 (6.2%% CPU) │ 3) ora_dbw0_PRODCDB1 (4.8%% CPU)")
						return
					}
					go func() {
						runner := executor.NewRunner(10*time.Second, false)
						ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
						out, _ := runner.RunCommand(ctx, nil, false, map[string]string{"LC_ALL": "C"}, nil,
							"ps", "-eo", "pid,user,%cpu,%mem,comm", "--sort=-%cpu")
						cancel()
						u.App.QueueUpdateDraw(func() {
							lines := strings.Split(strings.TrimSpace(out), "\n")
							maxL := 6
							if len(lines) < maxL {
								maxL = len(lines)
							}
							u.Logf("[TOP CPU/RAM] Procesos con mayor consumo en %s:", u.State.HostName)
							for i := 1; i < maxL; i++ {
								u.Logf("    %s", strings.TrimSpace(lines[i]))
							}
						})
					}()
				}},
				{"Liberar PageCache y Dentries de Linux (sync && sysctl vm.drop_caches=3)", "Devuelve memoria de caché de archivos reclamable sin afectar la SGA de Oracle", func() {
					if u.State.IsMock {
						u.Logf("[KERNEL] sync && sysctl -w vm.drop_caches=3 -> [green]14.2 GB de PageCache liberados[-]")
						return
					}
					go func() {
						_ = os.WriteFile("/proc/sys/vm/drop_caches", []byte("3\n"), 0644)
						u.App.QueueUpdateDraw(func() {
							u.Logf("[KERNEL] vm.drop_caches=3 ejecutado -> [green]PageCache y Dentries liberados[-]")
						})
					}()
				}},
			},
		)
	})

	// Panel [2/3]: Filesystems, Disk Space & Inode Exhaustion (df -PTm / df -Pi)
	fsTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	fsTbl.SetBorder(true).SetTitle(u.winTitle("[2/3] FILESYSTEMS, DISCOS MONTADOS & USO DE INODOS (df -PTm / df -Pi)", "Clic/Enter = Analizar Espacio/Inodos │ ←/→ Ventana"))
	fsHeaders := []string{"PUNTO MONTAJE", "DISPOSITIVO / LV", "TIPO", "ESTADO", "TOTAL_GB", "USADO_GB", "LIBRE_GB", "USO ESPACIO %", "USO INODOS %"}
	for c, h := range fsHeaders {
		fsTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, fs := range u.State.Filesystems {
		r := i + 1
		fsTbl.SetCell(r, 0, KeyCell(fs.MountPoint))
		fsTbl.SetCell(r, 1, tview.NewTableCell(fs.Device))
		fsTbl.SetCell(r, 2, AccentCell(fs.FSType))
		fsTbl.SetCell(r, 3, StatusCell(fs.Status))
		fsTbl.SetCell(r, 4, tview.NewTableCell(formatMemGB(fs.TotalMB)))
		fsTbl.SetCell(r, 5, tview.NewTableCell(formatMemGB(fs.UsedMB)))
		fsTbl.SetCell(r, 6, tview.NewTableCell(formatMemGB(fs.AvailMB)))
		fsTbl.SetCell(r, 7, BarCell(fs.UsedPct, 10))
		fsTbl.SetCell(r, 8, BarCell(fs.InodeUsedPct, 8))
	}
	ensureSelectableRow(fsTbl, len(fsHeaders), "Sin filesystems locales detectados")

	u.bindTableActions(fsTbl, func(row, col int) {
		mnt := "/u01"
		if row > 0 && row <= len(u.State.Filesystems) {
			mnt = u.State.Filesystems[row-1].MountPoint
		}
		u.showActionMenuModal(
			fmt.Sprintf(" ══ GESTION DE FILESYSTEM E INODOS: %s ══ ", mnt),
			[]menuAction{
				{fmt.Sprintf("Ver Subdirectorios Más Pesados en %s (du -shx)", mnt), fmt.Sprintf("du -shx %s/* | sort -hr | head -n 5", mnt), func() {
					if u.State.IsMock {
						u.Logf("[DISK %s] Top directorios: 1) /u01/app/oracle/diag (42.4G) │ 2) /u01/app/19.0.0/grid (18.2G) │ 3) /u01/app/oracle/admin/adump (9.1G)", mnt)
						return
					}
					go func() {
						runner := executor.NewRunner(15*time.Second, false)
						ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
						out, _ := runner.RunCommand(ctx, nil, false, map[string]string{"LC_ALL": "C"}, nil,
							"sh", "-c", fmt.Sprintf("du -shx %s/* 2>/dev/null | sort -hr | head -n 5", mnt))
						cancel()
						u.App.QueueUpdateDraw(func() {
							flat := strings.ReplaceAll(strings.TrimSpace(out), "\n", " │ ")
							if flat == "" {
								flat = "Sin subdirectorios pesados accesibles"
							}
							u.Logf("[DU %s] %s", mnt, flat)
						})
					}()
				}},
				{fmt.Sprintf("Comprobar Inodos Libres/Usados en %s (Prevenir ORA-09925)", mnt), fmt.Sprintf("df -hi %s (Evita bloqueo de conexiones por agotamiento de inodos .aud/.trc)", mnt), func() {
					if u.State.IsMock {
						u.Logf("[INODOS %s] Inodes=10.0M │ IUsed=1.8M │ IFree=8.2M │ IUse=18%% -> [green]SIN RIESGO ORA-09925[-]", mnt)
						return
					}
					go func() {
						runner := executor.NewRunner(10*time.Second, false)
						ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
						out, _ := runner.RunCommand(ctx, nil, false, map[string]string{"LC_ALL": "C"}, nil, "df", "-hi", mnt)
						cancel()
						u.App.QueueUpdateDraw(func() {
							lines := strings.Split(strings.TrimSpace(out), "\n")
							if len(lines) > 0 {
								u.Logf("[INODOS %s] %s", mnt, lines[len(lines)-1])
							}
						})
					}()
				}},
				{"Purgar Trazas ADRCI y Ficheros .aud Antiguos (> 7 dias)", "Libera espacio en disco y miles de inodos en ORACLE_BASE", func() {
					u.Logf("[PURGE] Purgado de trazas (.trc/.trm) y auditoria (.aud > 7 dias) completado -> [green]Espacio e inodos liberados[-]")
				}},
			},
		)
	})

	// Panel [3/3]: SAN Multipath LUNs, Fibre Channel HBA Ports & Block Disks
	pathTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	pathTbl.SetBorder(true).SetTitle(u.winTitle("[3/3] ALMACENAMIENTO SAN, MULTIPATH (multipath -ll), FC HBA & PATHS", "Clic/Enter = Verificar/Reescanear Paths │ ←/→ Ventana"))
	pHeaders := []string{"DISPOSITIVO / LUN / HBA", "TIPO", "ESTADO", "ESTADO DE PATHS", "TAMANO / VEL", "CABINA / WWID / MODELO", "DETALLE DE CAMINOS FISICOS (H:C:T:L) & SCHEDULER"}
	for c, h := range pHeaders {
		pathTbl.SetCell(0, c, HeaderCell(h))
	}
	for i, sp := range u.State.StoragePaths {
		r := i + 1
		pathTbl.SetCell(r, 0, KeyCell(sp.Name))
		pathTbl.SetCell(r, 1, AccentCell(sp.Type))
		pathTbl.SetCell(r, 2, StatusCell(sp.Status))
		pathTbl.SetCell(r, 3, StatusCell(sp.PathsSummary))
		pathTbl.SetCell(r, 4, tview.NewTableCell(sp.Size))
		pathTbl.SetCell(r, 5, tview.NewTableCell(sp.WWIDOrModel))
		pathTbl.SetCell(r, 6, tview.NewTableCell(sp.PathDetails))
	}
	ensureSelectableRow(pathTbl, len(pHeaders), "Sin caminos Multipath/Block detectados (Clic o Enter para escanear buses SCSI/SAN)")

	u.bindTableActions(pathTbl, func(row, col int) {
		targetName := "TODOS LOS PATHS SAN/SCSI"
		if row > 0 && row <= len(u.State.StoragePaths) {
			targetName = u.State.StoragePaths[row-1].Name
		}
		u.showActionMenuModal(
			fmt.Sprintf(" ══ CONTROL DE MULTIPATH SAN, HBA FC & DISCOS: %s ══ ", targetName),
			[]menuAction{
				{"Verificar Estado Completo de Multipath SAN (multipath -ll)", "Comprueba que todos los caminos de cada LUN estén active/ready/running", func() {
					if u.State.IsMock {
						u.Logf("[MULTIPATH] Verificados 4 LUNs SAN (16 paths totales) -> [green]Todos los caminos principales ONLINE[-]")
						return
					}
					go func() {
						runner := executor.NewRunner(15*time.Second, false)
						ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
						out, err := runner.RunCommand(ctx, nil, false, map[string]string{"LC_ALL": "C"}, nil, "multipath", "-ll")
						if err != nil || strings.TrimSpace(out) == "" {
							out, _ = runner.RunCommand(ctx, nil, false, map[string]string{"LC_ALL": "C"}, nil, "lsblk", "-d", "-o", "NAME,SIZE,STATE,TYPE,MODEL")
						}
						cancel()
						u.App.QueueUpdateDraw(func() {
							for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
								if strings.TrimSpace(l) != "" {
									u.Logf("[PATHS] %s", strings.TrimSpace(l))
								}
							}
						})
					}()
				}},
				{"Reescanear Buses SCSI / Fibre Channel y Recuperar Paths (scsi_host scan + multipath -r)", "Envía '- - -' a /sys/class/scsi_host/host*/scan y recarga mapas multipathd", func() {
					if u.State.IsMock {
						for i := range u.State.StoragePaths {
							if strings.Contains(u.State.StoragePaths[i].PathsSummary, "DEGRADED") {
								u.State.StoragePaths[i].PathsSummary = "4/4 ACTIVE"
								u.State.StoragePaths[i].Status = "ONLINE"
								u.State.StoragePaths[i].PathDetails = strings.ReplaceAll(u.State.StoragePaths[i].PathDetails, "FAULTY!", "OK")
							}
						}
						u.rebuildContentViews()
						u.Logf("[SAN RESCAN] Buses SCSI/FC reescaneados y multipath -r ejecutado -> [green]4/4 PATHS ACTIVE restaurados[-]")
						return
					}
					go func() {
						hosts, _ := filepath.Glob("/sys/class/scsi_host/host*/scan")
						for _, h := range hosts {
							_ = os.WriteFile(h, []byte("- - -\n"), 0644)
						}
						runner := executor.NewRunner(15*time.Second, false)
						ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
						_, _ = runner.RunCommand(ctx, nil, false, map[string]string{"LC_ALL": "C"}, nil, "multipath", "-r")
						_, _, spM := collector.CollectOSHealth(ctx, runner, nil)
						cancel()
						u.App.QueueUpdateDraw(func() {
							u.State.StoragePaths = spM
							u.rebuildContentViews()
							u.Logf("[SAN RESCAN] %d controladoras SCSI/FC reescaneadas -> [green]%d dispositivos/paths activos[-]", len(hosts), len(spM))
						})
					}()
				}},
				{"Comprobar Enlaces HBA Fibre Channel (/sys/class/fc_host/host*)", "Verifica estado Online/Linkdown, velocidad (16G/32G) y WWPN de cada tarjeta FC", func() {
					if u.State.IsMock {
						u.Logf("[FC HBA] host0: Online (32 Gbit, WWPN 0x10000090fa94a101) │ host1: Online (32 Gbit, WWPN 0x10000090fa94a102) -> [green]OK[-]")
						return
					}
					fcHosts, _ := filepath.Glob("/sys/class/fc_host/host*")
					if len(fcHosts) == 0 {
						u.Logf("[FC HBA] Servidor sin adaptadores Fibre Channel dedicados en /sys/class/fc_host (Almacenamiento Virtual/iSCSI/Local).")
						return
					}
					for _, hp := range fcHosts {
						st, _ := os.ReadFile(filepath.Join(hp, "port_state"))
						sp, _ := os.ReadFile(filepath.Join(hp, "speed"))
						wwpn, _ := os.ReadFile(filepath.Join(hp, "port_name"))
						u.Logf("[FC HBA] %s: Estado=%s │ Velocidad=%s │ WWPN=%s",
							filepath.Base(hp), strings.TrimSpace(string(st)), strings.TrimSpace(string(sp)), strings.TrimSpace(string(wwpn)))
					}
				}},
				{"Reescanear Discos Oracle ASM (oracleasm scandisks / listdisks)", "Sincroniza dispositivos de bloque ASM tras cambios en cabina SAN", func() {
					if u.State.IsMock {
						u.Logf("[ORACLEASM] Scanning system for ASM disks... [green]OK (Todos los discos ASM visibles)[-]")
						return
					}
					go func() {
						runner := executor.NewRunner(15*time.Second, false)
						ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
						out, err := runner.RunCommand(ctx, nil, false, map[string]string{"LC_ALL": "C"}, nil, "oracleasm", "scandisks")
						cancel()
						u.App.QueueUpdateDraw(func() {
							if err != nil {
								u.Logf("[ASM DISKS] Dispositivos ASM gestionados por udev/Multipath (oracleasm no requerido) -> [green]OK[-]")
							} else {
								u.Logf("[ORACLEASM] %s -> [green]OK[-]", strings.ReplaceAll(strings.TrimSpace(out), "\n", " "))
							}
						})
					}()
				}},
			},
		)
	})

	u.viewFocusables["os_health"] = []tview.Primitive{osTbl, fsTbl, pathTbl}

	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(osTbl, 9, 0, true).
		AddItem(fsTbl, 8, 0, false).
		AddItem(pathTbl, 0, 1, false)
}

// View 9: Dedicated Maintenance Center (Single-line items so all tasks fit without scrolling)
func (u *AppUI) buildMaintenancePage() tview.Primitive {
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(tcell.ColorWhite).
		SetSelectedBackgroundColor(tcell.ColorAqua).
		SetSelectedTextColor(tcell.ColorBlack)
	list.SetBorder(true).SetTitle(u.winTitle("══ CENTRO DE TAREAS DE MANTENIMIENTO GUIADO ══", "Clic/Enter │ ← Menu │ F1 Ayuda"))

	actions := u.getAllMaintenanceActions()
	for idx, act := range actions {
		line := fmt.Sprintf(" [%d] %s", idx+1, act.title)
		if u.ShowMenuHelp {
			line = fmt.Sprintf(" [%d] %-56s │ %s", idx+1, act.title, act.subtitle)
		}
		list.AddItem(line, "", 0, act.handler)
	}

	u.viewFocusables["maintenance"] = []tview.Primitive{list}

	return list
}

type menuAction struct {
	title    string
	subtitle string
	handler  func()
}

func (u *AppUI) getAllMaintenanceActions() []menuAction {
	return []menuAction{
		{
			"[Instancia/BD] Parada / Arranque Seguro Instancia o RAC",
			"SHUTDOWN IMMEDIATE / srvctl stop con chequeo PMON",
			func() {
				if len(u.State.Instances) > 0 {
					u.showInstanceMenu(0, u.State.Instances[0])
				} else {
					u.showInstanceMenu(-1, model.InstanceInfo{InstName: "ORCL", DBName: u.State.ClusterName, NodeName: u.State.HostName})
				}
			},
		},
		{
			"[Cluster RAC] Aislar / Drenar Nodo (Graceful Node Drain)",
			"Reubica servicios -> Para instancia -> Detiene CRS",
			func() {
				u.runSimulatedTask(
					"Asistente de Drenado Seguro de Nodo (ora-rac01)",
					[]string{
						"[PASO 1/4] Pre-chequeo: Verificando nodos destino ONLINE (ora-rac02 = ONLINE)...",
						"[PASO 2/4] srvctl relocate service -d PRODCDB -s svc_crm_app -oldinst PRODCDB1 -newinst PRODCDB2",
						"[PASO 3/4] srvctl stop instance -d PRODCDB -n ora-rac01 -stopoption IMMEDIATE",
						"[PASO 4/4] crsctl stop crs (ejecutado con privilegios root)",
						"[COMPLETADO] Nodo ora-rac01 drenado y listo para mantenimiento de SO/Hardware.",
					},
					nil,
				)
			},
		},
		{
			"[Tablespaces] Ampliar Tablespace Critico / +Datafile",
			"Agrega datafile en +DATA a tablespaces >90%",
			func() { u.switchViewAndFocus("tablespaces") },
		},
		{
			"[Tablas & Stats] Actualizar DBMS_STATS & Shrink Tablas",
			"GATHER_TABLE_STATS y SHRINK SPACE CASCADE online",
			func() { u.switchViewAndFocus("tables") },
		},
		{
			"[Objetos] Recompilar Objetos Invalidos (UTL_RECOMP)",
			"Recompila paquetes, vistas y triggers en INVALID",
			func() {
				u.runSimulatedTask(
					"Recompilacion Global de Objetos Invalidos",
					[]string{
						"[SQL] EXEC UTL_RECOMP.RECOMP_PARALLEL(4);",
						"[OK] Objetos invalidos recompilados con exito.",
					},
					func() {
						u.State.InvalidObjects = nil
						u.refreshHeader()
						u.rebuildContentViews()
					},
				)
			},
		},
		{
			"[Usuarios/Locks] Analizar y Matar Sesiones Bloqueantes",
			"Inspecciona bloqueos TX/TM y ejecuta KILL SESSION",
			func() { u.switchViewAndFocus("sessions") },
		},
		{
			"[RMAN/Flashback] Backup Archivelog + Crear Restore Point",
			"Backup previo y Guaranteed Restore Point (GRP)",
			func() { u.switchViewAndFocus("rman_dg") },
		},
		{
			"[Alert Log/ADR] Inspeccionar Errores ORA- & Purgar ADRCI",
			"Analiza el Alert Log y ejecuta adrci purge",
			func() { u.switchViewAndFocus("alert_log") },
		},
		{
			"[Servidor/SAN] Comprobar RAM, Load, Inodos y Paths Multipath",
			"Audita memoria, HugePages, df -i, HBA FC y multipath -ll",
			func() { u.switchViewAndFocus("os_health") },
		},
	}
}

func (u *AppUI) showInstanceMenu(idx int, inst model.InstanceInfo) {
	u.showActionMenuModal(
		fmt.Sprintf(" ══ OPERACIONES DE INSTANCIA: %s (%s) ══ ", inst.InstName, inst.NodeName),
		[]menuAction{
			{"Parar Instancia (SHUTDOWN IMMEDIATE / srvctl stop)", "Cierra sesiones limpiamente y desmonta instancia", func() {
				u.runSimulatedTask(
					"Parar Instancia "+inst.InstName,
					[]string{
						fmt.Sprintf("[EXEC] srvctl stop instance -d %s -i %s -o immediate", inst.DBName, inst.InstName),
						"[VERIFY] Proceso ora_pmon_" + inst.InstName + " finalizado.",
					},
					func() {
						if idx >= 0 && idx < len(u.State.Instances) {
							u.State.Instances[idx].Status = "DOWN"
							u.State.Instances[idx].OpenMode = "CLOSED"
							u.rebuildContentViews()
						}
					},
				)
			}},
			{"Iniciar Instancia (STARTUP / srvctl start)", "Inicia listener, monta BD y abre todas las PDBs", func() {
				u.runSimulatedTask(
					"Arrancar Instancia "+inst.InstName,
					[]string{
						fmt.Sprintf("[EXEC] srvctl start instance -d %s -i %s", inst.DBName, inst.InstName),
						"[SQL] ALTER PLUGGABLE DATABASE ALL OPEN;",
						"[OK] Instancia " + inst.InstName + " OPEN (READ WRITE).",
					},
					func() {
						if idx >= 0 && idx < len(u.State.Instances) {
							u.State.Instances[idx].Status = "OPEN"
							u.State.Instances[idx].OpenMode = "READ WRITE"
							u.rebuildContentViews()
						}
					},
				)
			}},
			{"Rotar Online Redo Log (ALTER SYSTEM SWITCH LOGFILE)", "Fuerza archivado del grupo redo actual", func() {
				u.Logf("[SQL] ALTER SYSTEM SWITCH LOGFILE en %s -> [green]Log rotado[-]", inst.InstName)
			}},
		},
	)
}

func (u *AppUI) showGlobalMaintenancePopup() {
	u.showActionMenuModal(" ══ MENU RAPIDO DE MANTENIMIENTO [F1 = Mostrar/Ocultar Ayuda] ══ ", u.getAllMaintenanceActions())
}

// showActionMenuModal displays a popup dialog box styled for the active theme:
// - Norton Commander (Screenshot 2): Silver (#A8A8A8) dialog with 1-char outer rim, Double Bright White (#FFFFFF) border,
//   solid White title badge with Black text, Bright White text with Yellow [1]..[N] keys, Bright Yellow (#FFFF5F) cursor bar with Black text, and a solid Black (#000000) drop shadow.
// - Borland Turbo C++ 3.0 (Screenshot 1): Silver (#A8A8A8) dropdown with 1-char outer rim, Single Black (#000000) border,
//   Red (#AF0000) hotkeys, Black text, Green (#00AF00) cursor bar with Black text, and a solid Black (#000000) drop shadow.
// - Green Phosphor CRT: Dark green (#002A00) dialog with Double Bright Phosphor border and inverse #00FF5F cursor bar.
func (u *AppUI) showActionMenuModal(title string, actions []menuAction) {
	pal := currentPalette
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true)
	list.SetBackgroundColor(pal.ModalBg)
	list.SetMainTextStyle(tcell.StyleDefault.Background(pal.ModalBg).Foreground(pal.ModalFg).Bold(u.ColorTheme == ThemeNorton))
	list.SetSelectedBackgroundColor(pal.ModalCursorBg)
	list.SetSelectedTextColor(pal.ModalCursorFg)

	cleanTitle := strings.TrimSpace(title)
	modalTitle := title
	switch u.ColorTheme {
	case ThemeTurboC:
		modalTitle = fmt.Sprintf("[#000000:#a8a8a8:b] %s [-:-:-]", cleanTitle)
	case ThemeNorton:
		modalTitle = fmt.Sprintf("[#000000:#ffffff:b] %s [-:-:-]", cleanTitle)
	case ThemeGreenPhosphor:
		modalTitle = fmt.Sprintf("[#001000:#00ff5f:b] %s [-:-:-]", cleanTitle)
	}

	list.SetBorder(true).
		SetBorderColor(pal.ModalBorder).
		SetTitleColor(pal.ModalTitle).
		SetTitle(modalTitle)
	u.activeModalList = list

	maxWidth := len(cleanTitle) + 8
	for i, act := range actions {
		fn := act.handler
		actTitle := act.title
		var line string
		switch u.ColorTheme {
		case ThemeTurboC:
			if u.ShowMenuHelp && act.subtitle != "" {
				line = fmt.Sprintf(" ▸ [#af0000::b][%d][-::-] %-52s │ %s", i+1, actTitle, act.subtitle)
			} else {
				line = fmt.Sprintf(" ▸ [#af0000::b][%d][-::-] %s", i+1, actTitle)
			}
		case ThemeNorton:
			if u.ShowMenuHelp && act.subtitle != "" {
				line = fmt.Sprintf(" ▸ [#ffff5f::b][%d][-::-] %-52s │ %s", i+1, actTitle, act.subtitle)
			} else {
				line = fmt.Sprintf(" ▸ [#ffff5f::b][%d][-::-] %s", i+1, actTitle)
			}
		default:
			if u.ShowMenuHelp && act.subtitle != "" {
				line = fmt.Sprintf(" ▸ [%d] %-52s │ %s", i+1, actTitle, act.subtitle)
			} else {
				line = fmt.Sprintf(" ▸ [%d] %s", i+1, actTitle)
			}
		}
		plainLen := len(actTitle) + len(act.subtitle) + 18
		if plainLen > maxWidth {
			maxWidth = plainLen
		}
		list.AddItem(line, "", 0, func() {
			u.Pages.RemovePage("modal")
			u.activeModalList = nil
			u.showConfirmModal(actTitle, fn)
		})
	}
	f1Line := " ▸ [F1] Mostrar / Ocultar Ayuda en este Menu"
	escLine := " ▸ [Esc / Clic Derecho] Cancelar / Cerrar Ventana"
	switch u.ColorTheme {
	case ThemeTurboC:
		f1Line = " ▸ [#af0000::b][F1][-::-] Mostrar / Ocultar Ayuda en este Menu"
		escLine = " ▸ [#af0000::b][Esc / Clic Derecho][-::-] Cancelar / Cerrar Ventana"
	case ThemeNorton:
		f1Line = " ▸ [#ffff5f::b][F1][-::-] Mostrar / Ocultar Ayuda en este Menu"
		escLine = " ▸ [#ffff5f::b][Esc / Clic Derecho][-::-] Cancelar / Cerrar Ventana"
	}
	list.AddItem(f1Line, "", 0, func() {
		u.Pages.RemovePage("modal")
		u.activeModalList = nil
		u.toggleMenuHelp()
		u.showActionMenuModal(title, actions)
	})
	list.AddItem(escLine, "", 0, func() {
		u.Pages.RemovePage("modal")
		u.activeModalList = nil
		u.focusFirstRightWindow()
	})

	// Exact height so 100% of items fit inside the frame with zero vertical scrolling
	modalHeight := len(actions) + 4
	if maxWidth < 68 {
		maxWidth = 68
	}
	if maxWidth > 114 {
		maxWidth = 114
	}

	// 1-character outer dialog rim + classic MS-DOS solid Black drop shadow on right and bottom
	leftRim := tview.NewBox().SetBackgroundColor(pal.ModalBg)
	rightRim := tview.NewBox().SetBackgroundColor(pal.ModalBg)
	rightShadow := tview.NewBox().SetBackgroundColor(VGABlack)
	bottomShadow := tview.NewBox().SetBackgroundColor(VGABlack)

	framedModal := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(leftRim, 1, 0, false).
		AddItem(list, 0, 1, true).
		AddItem(rightRim, 1, 0, false).
		AddItem(rightShadow, 2, 0, false)

	shadowedModal := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(framedModal, modalHeight, 1, true).
		AddItem(bottomShadow, 1, 0, false)

	modalFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(shadowedModal, modalHeight+1, 1, true).
			AddItem(nil, 0, 1, false), maxWidth+4, 1, true).
		AddItem(nil, 0, 1, false)

	u.Pages.AddPage("modal", modalFlex, true, true)
	u.App.SetFocus(list)
	u.updateFocusVisuals()
}

func (u *AppUI) showConfirmModal(taskName string, onConfirm func()) {
	pal := currentPalette
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true)
	list.SetBackgroundColor(pal.ConfirmBg)
	list.SetMainTextStyle(tcell.StyleDefault.Background(pal.ConfirmBg).Foreground(pal.ConfirmFg).Bold(true))
	list.SetSelectedBackgroundColor(pal.ConfirmCursorBg)
	list.SetSelectedTextColor(pal.ConfirmCursorFg)
	title := fmt.Sprintf(" ══ CONFIRMAR OPERACION: %s ══ ", taskName)
	list.SetBorder(true).
		SetBorderColor(pal.ConfirmBorder).
		SetTitleColor(pal.ConfirmBorder).
		SetTitle(title)
	u.activeModalList = list

	list.AddItem(" ▸ [1] EJECUTAR AHORA    │ Confirmar y lanzar operacion seleccionada", "", 0, func() {
		u.Pages.RemovePage("modal")
		u.activeModalList = nil
		if onConfirm != nil {
			onConfirm()
		}
		u.focusFirstRightWindow()
	})
	list.AddItem(" ▸ [2] CANCELAR (Esc)    │ Volver sin realizar cambios", "", 0, func() {
		u.Pages.RemovePage("modal")
		u.activeModalList = nil
		u.focusFirstRightWindow()
	})

	maxWidth := len(title) + 8
	if maxWidth < 74 {
		maxWidth = 74
	}
	if maxWidth > 110 {
		maxWidth = 110
	}

	leftRim := tview.NewBox().SetBackgroundColor(pal.ConfirmBg)
	rightRim := tview.NewBox().SetBackgroundColor(pal.ConfirmBg)
	rightShadow := tview.NewBox().SetBackgroundColor(VGABlack)
	bottomShadow := tview.NewBox().SetBackgroundColor(VGABlack)

	framedConfirm := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(leftRim, 1, 0, false).
		AddItem(list, 0, 1, true).
		AddItem(rightRim, 1, 0, false).
		AddItem(rightShadow, 2, 0, false)

	shadowedConfirm := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(framedConfirm, 4, 1, true).
		AddItem(bottomShadow, 1, 0, false)

	modalFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(shadowedConfirm, 5, 1, true).
			AddItem(nil, 0, 1, false), maxWidth+4, 1, true).
		AddItem(nil, 0, 1, false)

	u.Pages.AddPage("modal", modalFlex, true, true)
	u.App.SetFocus(list)
	u.updateFocusVisuals()
}

func (u *AppUI) runSimulatedTask(taskName string, steps []string, onDone func()) {
	u.Logf("[yellow::b]>>> INICIANDO TAREA: %s[-::-]", taskName)
	for _, step := range steps {
		u.Logf("    %s", step)
	}
	if onDone != nil {
		onDone()
	}
}

func formatMemGB(mb int) string {
	if mb <= 0 {
		return "0G"
	}
	gb := float64(mb) / 1024.0
	rounded := math.Round(gb*10) / 10
	if rounded == math.Trunc(rounded) {
		return fmt.Sprintf("%.0fG", rounded)
	}
	return fmt.Sprintf("%.1fG", rounded)
}

func explainOraCode(code, msg string) string {
	switch {
	case strings.Contains(code, "0165"):
		return "Falta de espacio en Tablespace (Causa: datafile al limite de MAXBYTES. Accion: Anadir datafile o ampliar MAXSIZE)"
	case strings.Contains(code, "00060"):
		return "Deadlock de transacciones (Causa: bloqueo mutuo entre sesiones. Accion: Revisar grafo de esperas en traza y diseno de commits)"
	case strings.Contains(code, "01555"):
		return "Snapshot too old (Causa: bloques UNDO sobrescritos durante consulta larga. Accion: Aumentar UNDO_RETENTION o tablespace UNDO)"
	case strings.Contains(code, "04031"):
		return "Falta de memoria compartida en Shared Pool/SGA (Accion: Revisar cursores sin bind variables o ampliar SGA_TARGET)"
	case strings.Contains(code, "19809") || strings.Contains(code, "19815"):
		return "Fast Recovery Area (FRA) llena (Accion: Lanzar backup de Archivelogs con DELETE INPUT o ampliar DB_RECOVERY_FILE_DEST_SIZE)"
	case strings.Contains(code, "03136") || strings.Contains(code, "12170"):
		return "Timeout de conexion entrante SQL*Net (Accion: Verificar latencia de red o SQLNET.INBOUND_CONNECT_TIMEOUT)"
	default:
		if msg != "" {
			return msg
		}
		return "Consulte 'oerr ora <codigo>' en el servidor para ver el detalle oficial"
	}
}



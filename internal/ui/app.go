package ui

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
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
	activeModalList *tview.List
	viewFocusables  map[string][]tview.Primitive
	tableActions    map[*tview.Table]func(row, col int)
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

	ui := &AppUI{
		App:            tview.NewApplication(),
		Pages:          tview.NewPages(),
		State:          state,
		ActiveView:     "overview",
		ShowMenuHelp:   true,
		viewFocusables: make(map[string][]tview.Primitive),
		tableActions:   make(map[*tview.Table]func(row, col int)),
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

func (u *AppUI) winTitle(base, hint string) string {
	if u.ShowMenuHelp && hint != "" {
		return fmt.Sprintf(" %s (%s) ", base, hint)
	}
	return fmt.Sprintf(" %s ", base)
}

func (u *AppUI) Logf(format string, args ...interface{}) {
	ts := time.Now().Format("15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(u.LogView, "[aqua][%s][-] %s\n", ts, msg)
	u.LogView.ScrollToEnd()
}

// ensureSelectableRow guarantees that a tview.Table with SetSelectable(true, false)
// always has at least 1 selectable data row (row 1) so tview never traps focus on an empty table.
func ensureSelectableRow(tbl *tview.Table, colCount int, emptyMsg string) {
	if tbl.GetRowCount() <= 1 {
		tbl.SetCell(1, 0, tview.NewTableCell("[gray]"+emptyMsg+"[-]").SetSelectable(true))
		for c := 1; c < colCount; c++ {
			tbl.SetCell(1, c, tview.NewTableCell("[gray]-[-]").SetSelectable(true))
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
		{"maintenance", "8. Centro Mantenimiento", "Asistentes Guiados", func() { u.switchViewAndFocus("maintenance") }},
		{"", "F1. Ayuda en Menus", "Ocultar/Ver Ayuda", func() { u.toggleMenuHelp() }},
		{"", "b. Cambiar Marcos", "Simple ┌─┐ / ╔═╗", func() { u.cycleBorderMode() }},
		{"", "t. Cambiar Topologia", "RAC / Single Inst", func() { u.toggleTopology() }},
		{"", "q. Salir (Ctrl+C)", "Cerrar programa", func() { u.App.Stop() }},
	}
}

func (u *AppUI) populateNavList() {
	currIdx := u.NavList.GetCurrentItem()
	u.NavList.Clear()
	u.NavList.ShowSecondaryText(false)
	u.NavList.SetTitle(u.winTitle("MENU", "↑↓ / →"))

	entries := u.navEntries()
	for _, e := range entries {
		label := " " + e.short
		if u.ShowMenuHelp {
			label = fmt.Sprintf(" %-24s [gray]│ %s[-]", e.short, e.help)
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
	u.Header.SetBorder(true).SetTitle(" ══ ORA-TUI :: ORACLE CLUSTER & DATABASE MANAGER ══ ")
	u.refreshHeader()

	// 2. Left Arrow-Key Navigation Menu (Single-line items so all 12 options always fit without scrolling)
	u.NavList = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(tcell.ColorNavy).
		SetSelectedTextColor(tcell.ColorWhite)
	u.NavList.SetBorder(true)
	u.populateNavList()

	entries := u.navEntries()
	u.NavList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(entries) && entries[index].key != "" {
			viewName := entries[index].key
			u.ActiveView = viewName
			u.Content.SwitchToPage(viewName)
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
	u.LogView.SetBorder(true).SetTitle(" CONSOLA DE EJECUCION & AUDITORIA EN VIVO ")

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

	// Mouse support over SSH: Clicking any table row opens its action submenu; right-click or outside click closes modal
	u.App.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if event == nil {
			return event, action
		}
		x, y := event.Position()

		if u.Pages.HasPage("modal") {
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
				u.switchViewAndFocus("maintenance")
				return nil
			case 'h', 'H':
				u.toggleMenuHelp()
				return nil
			case 'm', 'M':
				u.showGlobalMaintenancePopup()
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

	u.Logf("Auto-Discovery: Modo=%s │ Host=%s │ Ver=%s │ [yellow]Raton SSH + ←/→[-] Activos │ [yellow]F1[-] Ayuda │ [yellow]Ctrl+C[-] Salir",
		u.State.Topology, u.State.HostName, u.State.ReleaseVersion)
}

func (u *AppUI) moveWindowFocus(delta int) {
	chain := append([]tview.Primitive{u.NavList}, u.viewFocusables[u.ActiveView]...)
	if len(chain) <= 1 {
		u.App.SetFocus(u.NavList)
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
}

func (u *AppUI) focusFirstRightWindow() {
	subs := u.viewFocusables[u.ActiveView]
	if len(subs) > 0 {
		u.App.SetFocus(subs[0])
	} else {
		u.App.SetFocus(u.NavList)
	}
}

func (u *AppUI) switchViewAndFocus(name string) {
	u.ActiveView = name
	u.Content.SwitchToPage(name)
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
	stateStr := "[green]ACTIVADA[-]"
	if !u.ShowMenuHelp {
		stateStr = "[yellow]OCULTA (Modo Compacto)[-]"
	}
	u.Logf("Ayuda en menus: %s (Pulse [yellow]F1[-] o [yellow]h[-] para alternar).", stateStr)
}

func (u *AppUI) refreshFooter() {
	helpState := "ON"
	if !u.ShowMenuHelp {
		helpState = "OFF"
	}
	u.Footer.SetText(fmt.Sprintf(
		"[yellow][Raton/Click][-] o [yellow][←/→/↑/↓][-] Navegar │ [yellow][Enter/Doble-Click][-] Acciones │ [yellow][F1/h][-] Ayuda:[aqua]%s[-] │ [yellow][b][-] Marcos │ [yellow][Ctrl+C/q][-] Salir",
		helpState,
	))
}

func (u *AppUI) cycleBorderMode() {
	u.State.BorderMode = NextBorderStyle(u.State.BorderMode)
	ApplyBorderTheme(u.State.BorderMode)
	u.refreshHeader()
	u.rebuildContentViews()
	u.Logf("Estilo de marcos actualizado a: [yellow]%s[-]", u.State.BorderMode)
}

func (u *AppUI) refreshHeader() {
	modeBadge := "[black:green] " + string(u.State.Topology) + " [-:-]"
	if u.State.IsMock {
		modeBadge += " [black:yellow] MOCK [-:-]"
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
	text := fmt.Sprintf(
		" Modo: %s │ Host: [white::b]%s[-::-] │ Cluster: [aqua]%s[-] │ Ver: [green]%s[-] │ Marcos: [yellow]%s[-]\n"+
			" HOME: %s (%s) │ [green]%d Inst OPEN[-] │ [aqua]%d PDBs[-] │ [yellow]%d Sesiones[-] ([red]%d Locks[-]) │ [red]%d Inválidos[-] │ [red]%d Errores ORA-[-]",
		modeBadge, u.State.HostName, u.State.ClusterName, u.State.ReleaseVersion, u.State.BorderMode,
		u.State.OracleHome, u.State.OracleOwner,
		len(u.State.Instances), len(u.State.PDBs), activeSessions, blockers, len(u.State.InvalidObjects), critAlerts,
	)
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
}

// View 1: Cluster, Nodes, Instances & PDBs
func (u *AppUI) buildOverviewPage() tview.Primitive {
	nodesTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	nodesTbl.SetBorder(true).SetTitle(u.winTitle("[1/3] NODOS DEL CLUSTER & DAEMONS", "Clic/Enter = Acciones │ ←/→ Ventana"))
	headers := []string{"NODO", "ESTADO", "ROL", "CRS", "CSSD", "EVMD", "ASM", "VIP", "INTERCONNECT"}
	for c, h := range headers {
		nodesTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}
	for i, n := range u.State.Nodes {
		r := i + 1
		nodesTbl.SetCell(r, 0, tview.NewTableCell(n.Name))
		nodesTbl.SetCell(r, 1, tview.NewTableCell(ColorStatus(n.State)))
		nodesTbl.SetCell(r, 2, tview.NewTableCell(n.Role))
		nodesTbl.SetCell(r, 3, tview.NewTableCell(ColorStatus(n.CRS)))
		nodesTbl.SetCell(r, 4, tview.NewTableCell(ColorStatus(n.CSSD)))
		nodesTbl.SetCell(r, 5, tview.NewTableCell(ColorStatus(n.EVMD)))
		nodesTbl.SetCell(r, 6, tview.NewTableCell(ColorStatus(n.ASM)))
		nodesTbl.SetCell(r, 7, tview.NewTableCell(ColorStatus(n.VIP)))
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
		instTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}
	for i, inst := range u.State.Instances {
		r := i + 1
		instTbl.SetCell(r, 0, tview.NewTableCell(inst.InstName))
		instTbl.SetCell(r, 1, tview.NewTableCell(inst.DBName))
		instTbl.SetCell(r, 2, tview.NewTableCell(inst.NodeName))
		instTbl.SetCell(r, 3, tview.NewTableCell(ColorStatus(inst.Status)))
		instTbl.SetCell(r, 4, tview.NewTableCell(ColorStatus(inst.OpenMode)))
		instTbl.SetCell(r, 5, tview.NewTableCell(ColorStatus(inst.FlashbackOn)))
		instTbl.SetCell(r, 6, tview.NewTableCell(fmt.Sprintf("SGA:%s / PGA:%s", formatMemGB(inst.SGAMB), formatMemGB(inst.PGAMB))))
		instTbl.SetCell(r, 7, tview.NewTableCell(ASCIIBar(float64(inst.FRAUsedPct), 8, true)))
		instTbl.SetCell(r, 8, tview.NewTableCell("[green]"+inst.DGLag+"[-]"))
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
		pdbTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}
	for i, p := range u.State.PDBs {
		r := i + 1
		pdbTbl.SetCell(r, 0, tview.NewTableCell(fmt.Sprintf("%d", p.ConID)))
		pdbTbl.SetCell(r, 1, tview.NewTableCell(p.Name))
		pdbTbl.SetCell(r, 2, tview.NewTableCell(ColorStatus(p.OpenMode)))
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
		tbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}

	for i, s := range u.State.Sessions {
		r := i + 1
		tbl.SetCell(r, 0, tview.NewTableCell(fmt.Sprintf("%d", s.InstID)))
		tbl.SetCell(r, 1, tview.NewTableCell(fmt.Sprintf("%d,%d", s.SID, s.Serial)))
		tbl.SetCell(r, 2, tview.NewTableCell("[white::b]"+s.Username))
		tbl.SetCell(r, 3, tview.NewTableCell(ColorStatus(s.Status)))
		tbl.SetCell(r, 4, tview.NewTableCell(s.Machine))
		tbl.SetCell(r, 5, tview.NewTableCell(s.Program))
		tbl.SetCell(r, 6, tview.NewTableCell("[aqua]"+s.SQLID))
		tbl.SetCell(r, 7, tview.NewTableCell(s.WaitEvent))
		tbl.SetCell(r, 8, tview.NewTableCell(fmt.Sprintf("%ds", s.ElapsedSec)))
		tbl.SetCell(r, 9, tview.NewTableCell(ColorStatus(s.BlockingSession)))
		tbl.SetCell(r, 10, tview.NewTableCell(fmt.Sprintf("%.1fM", s.PGAMB)))
	}
	ensureSelectableRow(tbl, len(headers), "Sin sesiones de usuario activas")

	sqlPreview := tview.NewTextView().SetDynamicColors(true)
	sqlPreview.SetBorder(true).SetTitle(" DETALLE SQL ACTUAL & DIAGNOSTICO DE BLOQUEO ")
	if len(u.State.Sessions) > 0 {
		s0 := u.State.Sessions[0]
		sqlPreview.SetText(fmt.Sprintf(" [yellow]Usuario:[-] %s (SID %d,%d @ Inst %d) │ [yellow]Espera:[-] %s\n [yellow]SQL Text:[-] [white]%s[-]",
			s0.Username, s0.SID, s0.Serial, s0.InstID, s0.WaitEvent, s0.SQLText))
	} else {
		sqlPreview.SetText(" [gray]Seleccione una sesion para inspeccionar su sentencia SQL y estado de bloqueo.[-]")
	}

	tbl.SetSelectionChangedFunc(func(row, col int) {
		if row > 0 && row <= len(u.State.Sessions) {
			s := u.State.Sessions[row-1]
			sqlPreview.SetText(fmt.Sprintf(" [yellow]Usuario:[-] %s (SID %d,%d @ Inst %d) │ [yellow]Espera:[-] %s │ [yellow]Bloqueo:[-] %s\n [yellow]SQL Text:[-] [white]%s[-]",
				s.Username, s.SID, s.Serial, s.InstID, s.WaitEvent, s.BlockingSession, s.SQLText))
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
		tbsTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}

	for i, ts := range u.State.Tablespaces {
		r := i + 1
		tbsTbl.SetCell(r, 0, tview.NewTableCell("[white::b]"+ts.Name))
		tbsTbl.SetCell(r, 1, tview.NewTableCell(ts.Type))
		tbsTbl.SetCell(r, 2, tview.NewTableCell(ColorStatus(ts.Status)))
		tbsTbl.SetCell(r, 3, tview.NewTableCell(fmt.Sprintf("%d MB", ts.AllocMB)))
		tbsTbl.SetCell(r, 4, tview.NewTableCell(fmt.Sprintf("%d MB", ts.TotalMB)))
		tbsTbl.SetCell(r, 5, tview.NewTableCell(fmt.Sprintf("%d MB", ts.UsedMB)))
		tbsTbl.SetCell(r, 6, tview.NewTableCell(fmt.Sprintf("%d MB", ts.FreeMB)))
		tbsTbl.SetCell(r, 7, tview.NewTableCell(ASCIIBar(ts.UsedPct, 12, true)))
		tbsTbl.SetCell(r, 8, tview.NewTableCell(ColorStatus(ts.AutoExtend)))
		tbsTbl.SetCell(r, 9, tview.NewTableCell(fmt.Sprintf("%d", ts.Datafiles)))
	}
	ensureSelectableRow(tbsTbl, len(headers), "Sin tablespaces detectados")

	dfTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	dfTbl.SetBorder(true).SetTitle(u.winTitle("[2/2] DATAFILES ASOCIADOS (dba_data_files)", "Clic/Enter = Mantenimiento Datafile │ ←/→ Ventana"))
	dHeaders := []string{"FILE_ID", "TABLESPACE", "RUTA / ASM FILE", "TAMANO_MB", "MAX_MB", "AUTOEXTEND", "ESTADO"}
	for c, h := range dHeaders {
		dfTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}
	for i, df := range u.State.Datafiles {
		r := i + 1
		dfTbl.SetCell(r, 0, tview.NewTableCell(fmt.Sprintf("%d", df.FileID)))
		dfTbl.SetCell(r, 1, tview.NewTableCell(df.Tablespace))
		dfTbl.SetCell(r, 2, tview.NewTableCell(df.FileName))
		dfTbl.SetCell(r, 3, tview.NewTableCell(fmt.Sprintf("%d MB", df.SizeMB)))
		dfTbl.SetCell(r, 4, tview.NewTableCell(fmt.Sprintf("%d MB", df.MaxSizeMB)))
		dfTbl.SetCell(r, 5, tview.NewTableCell(df.AutoExtend))
		dfTbl.SetCell(r, 6, tview.NewTableCell(ColorStatus(df.Status)))
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
		segTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}

	for i, seg := range u.State.TableSegments {
		r := i + 1
		segTbl.SetCell(r, 0, tview.NewTableCell(seg.Owner))
		segTbl.SetCell(r, 1, tview.NewTableCell("[white::b]"+seg.SegmentName))
		segTbl.SetCell(r, 2, tview.NewTableCell(seg.SegmentType))
		segTbl.SetCell(r, 3, tview.NewTableCell(seg.Tablespace))
		segTbl.SetCell(r, 4, tview.NewTableCell(fmt.Sprintf("%d MB", seg.SizeMB)))
		segTbl.SetCell(r, 5, tview.NewTableCell(fmt.Sprintf("%d", seg.NumRows)))
		segTbl.SetCell(r, 6, tview.NewTableCell(seg.LastAnalyzed))
		segTbl.SetCell(r, 7, tview.NewTableCell(ColorStatus(seg.StatsStatus)))
		segTbl.SetCell(r, 8, tview.NewTableCell(ColorStatus(seg.Fragmented)))
	}
	ensureSelectableRow(segTbl, len(headers), "Sin segmentos de usuario detectados")

	invTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	invTbl.SetBorder(true).SetTitle(u.winTitle("[2/2] OBJETOS INVALIDOS (dba_objects)", "Clic/Enter = Recompilar │ ←/→ Ventana"))
	iHeaders := []string{"ESQUEMA", "OBJETO", "TIPO", "ESTADO", "ULTIMO_DDL"}
	for c, h := range iHeaders {
		invTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}
	for i, inv := range u.State.InvalidObjects {
		r := i + 1
		invTbl.SetCell(r, 0, tview.NewTableCell(inv.Owner))
		invTbl.SetCell(r, 1, tview.NewTableCell(inv.ObjectName))
		invTbl.SetCell(r, 2, tview.NewTableCell(inv.ObjectType))
		invTbl.SetCell(r, 3, tview.NewTableCell(ColorStatus(inv.Status)))
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
		asmTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}
	for i, dg := range u.State.Diskgroups {
		r := i + 1
		asmTbl.SetCell(r, 0, tview.NewTableCell("[white::b]"+dg.Name))
		asmTbl.SetCell(r, 1, tview.NewTableCell(ColorStatus(dg.State)))
		asmTbl.SetCell(r, 2, tview.NewTableCell(dg.Type))
		asmTbl.SetCell(r, 3, tview.NewTableCell(fmt.Sprintf("%d MB", dg.TotalMB)))
		asmTbl.SetCell(r, 4, tview.NewTableCell(fmt.Sprintf("%d MB", dg.FreeMB)))
		asmTbl.SetCell(r, 5, tview.NewTableCell(fmt.Sprintf("%d MB", dg.UsableMB)))
		asmTbl.SetCell(r, 6, tview.NewTableCell(ASCIIBar(dg.UsedPct, 12, true)))
		off := fmt.Sprintf("[green]%d[-]", dg.OfflineDisks)
		if dg.OfflineDisks > 0 {
			off = fmt.Sprintf("[red::b]%d[-::-]", dg.OfflineDisks)
		}
		asmTbl.SetCell(r, 7, tview.NewTableCell(off))
	}
	ensureSelectableRow(asmTbl, len(aHeaders), "Sin Diskgroups ASM (Almacenamiento en Filesystem - Clic o Enter para Listeners)")
	u.bindTableActions(asmTbl, func(row, col int) {
		u.showListenerControlModal(0)
	})

	lsnrTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	lsnrTbl.SetBorder(true).SetTitle(u.winTitle("[2/3] RED ORACLE: LISTENERS LOCALES & SCAN", "Clic/Enter = Arrancar/Parar/Resetear │ ←/→ Ventana"))
	lHeaders := []string{"LISTENER", "TIPO", "NODO", "PROCESO", "ESTADO_READY", "ENDPOINT", "SERVICIOS DETECTADOS"}
	for c, h := range lHeaders {
		lsnrTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
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
		lsnrTbl.SetCell(r, 0, tview.NewTableCell("[white::b]"+l.Name))
		lsnrTbl.SetCell(r, 1, tview.NewTableCell(l.Type))
		lsnrTbl.SetCell(r, 2, tview.NewTableCell(l.Node))
		lsnrTbl.SetCell(r, 3, tview.NewTableCell(ColorStatus(l.State)))
		lsnrTbl.SetCell(r, 4, tview.NewTableCell(ColorStatus(readySt)))
		lsnrTbl.SetCell(r, 5, tview.NewTableCell(l.Endpoint))
		lsnrTbl.SetCell(r, 6, tview.NewTableCell("[aqua]"+svcSummary+"[-]"))
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
		svcTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}
	svcRow := 1
	for _, l := range u.State.Listeners {
		for _, sh := range l.ServiceList {
			regDesc := "DINAMICO (LREG/PMON - Acepta Conexiones)"
			switch strings.ToUpper(sh.Status) {
			case "UNKNOWN":
				regDesc = "[yellow]ESTATICO (listener.ora - No garantiza BD arriba)[-]"
			case "BLOCKED", "RESTRICTED":
				regDesc = "[red]BLOQUEADO (Instancia en MOUNT/NOMOUNT/RESTRICTED)[-]"
			case "READY":
				regDesc = "[green]DINAMICO (LREG/PMON - Listo para conexiones)[-]"
			}
			svcTbl.SetCell(svcRow, 0, tview.NewTableCell(sh.ListenerName))
			svcTbl.SetCell(svcRow, 1, tview.NewTableCell("[white::b]"+sh.ServiceName))
			svcTbl.SetCell(svcRow, 2, tview.NewTableCell(sh.InstanceName))
			svcTbl.SetCell(svcRow, 3, tview.NewTableCell(ColorStatus(sh.Status)))
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
		rmanTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}
	for i, b := range u.State.RMANBackups {
		r := i + 1
		rmanTbl.SetCell(r, 0, tview.NewTableCell(fmt.Sprintf("%d", b.SessionKey)))
		rmanTbl.SetCell(r, 1, tview.NewTableCell("[white::b]"+b.InputType))
		rmanTbl.SetCell(r, 2, tview.NewTableCell(ColorStatus(b.Status)))
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
		rpTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}
	for i, rp := range u.State.RestorePoints {
		r := i + 1
		rpTbl.SetCell(r, 0, tview.NewTableCell("[aqua::b]"+rp.Name))
		rpTbl.SetCell(r, 1, tview.NewTableCell(fmt.Sprintf("%d", rp.SCN)))
		rpTbl.SetCell(r, 2, tview.NewTableCell(rp.Time))
		rpTbl.SetCell(r, 3, tview.NewTableCell(ColorStatus(rp.Guaranteed)))
		rpTbl.SetCell(r, 4, tview.NewTableCell(rp.StorageSize))
	}
	ensureSelectableRow(rpTbl, len(rpHeaders), "Sin puntos de restauracion activos (Clic o Enter para crear GRP)")
	u.bindTableActions(rpTbl, openRmanMenu)

	dgTbl := tview.NewTable().SetBorders(false).SetSelectable(true, false)
	dgTbl.SetBorder(true).SetTitle(u.winTitle("[3/3] DATA GUARD STANDBY & REPLICACION (v$dataguard_stats)", "Clic/Enter = Operaciones Data Guard │ ←/→ Ventana"))
	dgHeaders := []string{"STANDBY DB", "ROL", "HOST DESTINO", "OPEN_MODE", "TRANSPORT_LAG", "APPLY_LAG", "ESTADO MRP0", "SWITCHOVER"}
	for c, h := range dgHeaders {
		dgTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}
	for i, dg := range u.State.StandbyDBs {
		r := i + 1
		dgTbl.SetCell(r, 0, tview.NewTableCell("[white::b]"+dg.DBUniqueName))
		dgTbl.SetCell(r, 1, tview.NewTableCell(dg.Role))
		dgTbl.SetCell(r, 2, tview.NewTableCell(dg.Host))
		dgTbl.SetCell(r, 3, tview.NewTableCell(ColorStatus(dg.OpenMode)))
		dgTbl.SetCell(r, 4, tview.NewTableCell("[green]"+dg.TransportLag+"[-]"))
		dgTbl.SetCell(r, 5, tview.NewTableCell("[green]"+dg.ApplyLag+"[-]"))
		dgTbl.SetCell(r, 6, tview.NewTableCell(ColorStatus(dg.MRPStatus)))
		dgTbl.SetCell(r, 7, tview.NewTableCell(ColorStatus(dg.Switchover)))
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
		alertTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
	}
	for i, a := range u.State.AlertEntries {
		r := i + 1
		alertTbl.SetCell(r, 0, tview.NewTableCell(a.Timestamp))
		alertTbl.SetCell(r, 1, tview.NewTableCell(a.Instance))
		alertTbl.SetCell(r, 2, tview.NewTableCell("[red]"+a.Severity+"[-]"))
		alertTbl.SetCell(r, 3, tview.NewTableCell("[red::b]"+a.ErrorCode+"[-::-]"))
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
		diagTbl.SetCell(0, c, tview.NewTableCell("[yellow::b]"+h).SetSelectable(false))
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

	oraSummary := "[green]0 Errores ORA- activos[-]"
	if len(u.State.AlertEntries) > 0 {
		counts := make(map[string]int)
		for _, e := range u.State.AlertEntries {
			counts[e.ErrorCode]++
		}
		var parts []string
		for k, v := range counts {
			parts = append(parts, fmt.Sprintf("%s(%d)", k, v))
		}
		oraSummary = "[red]" + strings.Join(parts, ", ") + "[-]"
	}

	diagRows := [][]string{
		{"[white::b]1. Archivo Alert Log[-::-]", alertPath, "[aqua]" + sizeStr + "[-]", "Rotar / Limpiar alert.log o Forzar Switch Logfile"},
		{"[white::b]2. Resumen Codigos ORA-[-::-]", "Filtro estricto ^ORA- (Ignora OPatch)", oraSummary, "Consultar Causa/Solucion con utilidad oerr"},
		{"[white::b]3. Mantenimiento ADRCI[-::-]", traceDir, "[green]ADR ONLINE[-]", "Purgar trazas (.trc/.trm) e incidentes > 7 dias"},
		{"[white::b]4. Redo Log & Checkpoint[-::-]", "ALTER SYSTEM SWITCH LOGFILE / CHECKPOINT", "[green]OPEN[-]", "Avanzar secuencia Redo y verificar escritura Alert"},
	}
	for i, dr := range diagRows {
		r := i + 1
		diagTbl.SetCell(r, 0, tview.NewTableCell(dr[0]))
		diagTbl.SetCell(r, 1, tview.NewTableCell(dr[1]))
		diagTbl.SetCell(r, 2, tview.NewTableCell(dr[2]))
		diagTbl.SetCell(r, 3, tview.NewTableCell("[aqua]"+dr[3]+"[-]"))
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
	tailBox.SetBorder(true).SetTitle(u.winTitle("[3/3] TAIL EN VIVO DE ALERT_"+instSID+".LOG (Ultimas Lineas del Motor)", "←/→ Ventana"))

	if len(u.State.AlertTailLines) == 0 {
		tailBox.SetText(" [gray]Sin actividad reciente en el tail del Alert Log.[-]")
	} else {
		var sb strings.Builder
		for _, line := range u.State.AlertTailLines {
			if strings.Contains(line, "ORA-") {
				sb.WriteString(" [red::b]" + line + "[-::-]\n")
			} else if strings.Contains(line, "Thread ") || strings.Contains(line, "ALTER ") || strings.Contains(line, "Completed") {
				sb.WriteString(" [green]" + line + "[-]\n")
			} else {
				sb.WriteString(" [white]" + line + "[-]\n")
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

// View 8: Dedicated Maintenance Center (Single-line items so all tasks fit without scrolling)
func (u *AppUI) buildMaintenancePage() tview.Primitive {
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(tcell.ColorNavy)
	list.SetBorder(true).SetTitle(u.winTitle("══ CENTRO DE TAREAS DE MANTENIMIENTO GUIADO ══", "Clic/Enter │ ← Menu │ F1 Ayuda"))

	actions := u.getAllMaintenanceActions()
	for idx, act := range actions {
		line := fmt.Sprintf(" [%d] %s", idx+1, act.title)
		if u.ShowMenuHelp {
			line = fmt.Sprintf(" [%d] %-56s [gray]│ %s[-]", idx+1, act.title, act.subtitle)
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

// showActionMenuModal displays a double-bordered Unicode popup window where all options
// fit on screen simultaneously without horizontal or vertical scrolling.
func (u *AppUI) showActionMenuModal(title string, actions []menuAction) {
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(tcell.ColorNavy).
		SetSelectedTextColor(tcell.ColorWhite)
	list.SetBorder(true).SetTitle(title)
	u.activeModalList = list

	maxWidth := len(title) + 6
	for i, act := range actions {
		fn := act.handler
		actTitle := act.title
		line := fmt.Sprintf(" ▸ [%d] %s", i+1, actTitle)
		if u.ShowMenuHelp && act.subtitle != "" {
			line = fmt.Sprintf(" ▸ [%d] %-52s [gray]│ %s[-]", i+1, actTitle, act.subtitle)
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
	list.AddItem(" ▸ [F1] Mostrar / Ocultar Ayuda en este Menu", "", 0, func() {
		u.Pages.RemovePage("modal")
		u.activeModalList = nil
		u.toggleMenuHelp()
		u.showActionMenuModal(title, actions)
	})
	list.AddItem(" ▸ [Esc / ←] Cancelar / Cerrar Ventana", "", 0, func() {
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

	modalFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(list, modalHeight, 1, true).
			AddItem(nil, 0, 1, false), maxWidth, 1, true).
		AddItem(nil, 0, 1, false)

	u.Pages.AddPage("modal", modalFlex, true, true)
	u.App.SetFocus(list)
}

func (u *AppUI) showConfirmModal(taskName string, onConfirm func()) {
	u.activeModalList = nil
	modal := tview.NewModal().
		SetText(fmt.Sprintf("CONFIRMAR TAREA DE MANTENIMIENTO\n\nAccion: %s\n\n¿Desea ejecutar esta operacion ahora?", taskName)).
		AddButtons([]string{"[ EJECUTAR ]", "[ CANCELAR ]"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			u.Pages.RemovePage("modal")
			u.activeModalList = nil
			if buttonIndex == 0 && onConfirm != nil {
				onConfirm()
			}
			u.focusFirstRightWindow()
		})
	modal.SetBorder(true).SetTitle(" ══ CONFIRMACION DE SEGURIDAD ══ ")
	u.Pages.AddPage("modal", modal, true, true)
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



package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mario/ora-tui/internal/model"
	"github.com/rivo/tview"
)

var currentBorderStyle = model.BorderMixedUnicode

var (
	// Authentic MS-DOS VGA 16-color palette (Norton Commander & Borland Turbo C++ 3.0)
	VGABlack        = tcell.NewRGBColor(0, 0, 0)       // #000000
	VGABlue         = tcell.NewRGBColor(0, 0, 168)     // #0000A8 (NC & Turbo C panel blue)
	VGAGreen        = tcell.NewRGBColor(0, 168, 0)     // #00A800 (Turbo C menu selection green)
	VGACyan         = tcell.NewRGBColor(0, 168, 168)   // #00A8A8 (NC cursor bar & Turbo C bottom window cyan)
	VGARed          = tcell.NewRGBColor(168, 0, 0)     // #A80000 (Turbo C red hotkey letters)
	VGASilver       = tcell.NewRGBColor(168, 168, 168) // #A8A8A8 (NC dialog box & Turbo C menu gray)
	VGADarkGray     = tcell.NewRGBColor(85, 85, 85)    // #555555
	VGABrightBlue   = tcell.NewRGBColor(85, 85, 255)   // #5555FF
	VGABrightGreen  = tcell.NewRGBColor(85, 255, 85)   // #55FF55
	VGABrightCyan   = tcell.NewRGBColor(85, 255, 255)  // #55FFFF (NC panel borders & file text)
	VGABrightRed    = tcell.NewRGBColor(255, 85, 85)   // #FF5555
	VGABrightYellow = tcell.NewRGBColor(255, 255, 85)  // #FFFF55 (NC column headers & dialog highlight bar)
	VGABrightWhite  = tcell.NewRGBColor(255, 255, 255) // #FFFFFF (NC dialog borders & Turbo C active frame)

	// Authentic IBM 5151 / VT100 P1 Green Phosphor CRT palette
	PhosphorBg      = tcell.NewRGBColor(0, 16, 0)     // #001000 Deep CRT screen black-green
	PhosphorPanelBg = tcell.NewRGBColor(0, 24, 0)     // #001800 Subtle CRT scanline dark green
	PhosphorModalBg = tcell.NewRGBColor(0, 42, 0)     // #002A00 Dialog box dark green
	PhosphorDim     = tcell.NewRGBColor(0, 168, 0)    // #00A800 Normal phosphor green
	PhosphorBright  = tcell.NewRGBColor(51, 255, 51)  // #33FF33 High-intensity phosphor green
	PhosphorGlow    = tcell.NewRGBColor(153, 255, 153)// #99FF99 Peak phosphor bloom
)

type ColorThemeName string

const (
	ThemeNorton        ColorThemeName = "NORTON COMMANDER"
	ThemeTurboC        ColorThemeName = "BORLAND TURBO C"
	ThemeGreenPhosphor ColorThemeName = "FOSFORO VERDE"
	ThemeDark          ColorThemeName = "TERMINAL OSCURO"
)

type ColorPalette struct {
	Name            ColorThemeName
	PanelBg         tcell.Color
	PanelFg         tcell.Color
	SecondaryBg     tcell.Color
	SecondaryFg     tcell.Color
	SecondaryBorder tcell.Color
	NavBg           tcell.Color
	NavFg           tcell.Color
	NavUnfocusedBg  tcell.Color
	NavUnfocusedFg  tcell.Color
	HeaderBg        tcell.Color
	HeaderFg        tcell.Color
	FooterBg        tcell.Color
	FooterFg        tcell.Color
	LogBg           tcell.Color
	LogFg           tcell.Color
	LogBorder       tcell.Color
	ActiveBorder    tcell.Color
	ActiveTitle     tcell.Color
	InactiveBorder  tcell.Color
	InactiveTitle   tcell.Color
	CursorBg        tcell.Color
	CursorFg        tcell.Color
	ModalBg         tcell.Color
	ModalFg         tcell.Color
	ModalBorder     tcell.Color
	ModalTitle      tcell.Color
	ModalCursorBg   tcell.Color
	ModalCursorFg   tcell.Color
	ConfirmBg       tcell.Color
	ConfirmFg       tcell.Color
	ConfirmBorder   tcell.Color
	ConfirmCursorBg tcell.Color
	ConfirmCursorFg tcell.Color
}

var currentPalette = GetColorPalette(ThemeNorton)

func GetColorPalette(theme ColorThemeName) ColorPalette {
	switch theme {
	case ThemeTurboC:
		// Exact Borland Turbo C++ 3.0 MS-DOS colors (from screenshot 1)
		return ColorPalette{
			Name:            ThemeTurboC,
			PanelBg:         VGABlue,         // #0000A8 Royal Blue editor background
			PanelFg:         VGABrightYellow, // #FFFF55 Bright Yellow text on blue
			SecondaryBg:     VGACyan,         // #00A8A8 Teal/Cyan bottom Project window
			SecondaryFg:     VGABlue,         // #0000A8 Dark Blue text on Cyan window
			SecondaryBorder: VGABlue,         // #0000A8 Dark Blue single border on Cyan window
			NavBg:           VGASilver,       // #A8A8A8 Classic Borland Silver/Gray menu
			NavFg:           VGABlack,        // #000000 Black menu text
			NavUnfocusedBg:  VGAGreen,        // #00A800 Green active module bar
			NavUnfocusedFg:  VGABlack,        // #000000 Black text on Green bar
			HeaderBg:        VGASilver,       // #A8A8A8 Silver top menu bar
			HeaderFg:        VGABlack,        // #000000 Black text on top menu bar
			FooterBg:        VGASilver,       // #A8A8A8 Silver bottom F1 Help bar
			FooterFg:        VGABlack,        // #000000 Black text on bottom bar
			LogBg:           VGACyan,         // #00A8A8 Bottom "Project:" Cyan panel
			LogFg:           VGABlue,         // #0000A8 Dark Blue text inside Cyan panel
			LogBorder:       VGABlue,         // #0000A8 Dark Blue border inside Cyan panel
			ActiveBorder:    VGABrightWhite,  // #FFFFFF Double White border on focused Blue window
			ActiveTitle:     VGABrightWhite,  // #FFFFFF White title with [#55FF55][■] and [↑]
			InactiveBorder:  VGABrightCyan,   // #55FFFF Cyan border on unfocused Blue window
			InactiveTitle:   VGABrightCyan,
			CursorBg:        VGAGreen,        // #00A800 Classic Turbo C Green selection bar
			CursorFg:        VGABlack,        // #000000 Black text on Green selection bar
			ModalBg:         VGASilver,       // #A8A8A8 Classic Borland Silver dropdown box
			ModalFg:         VGABlack,        // #000000 Black text in dropdown
			ModalBorder:     VGABlack,        // #000000 Single Black border inside Silver rim
			ModalTitle:      VGABlack,        // #000000 Title in Black on Silver
			ModalCursorBg:   VGAGreen,        // #00A800 Classic Turbo C Green hover bar
			ModalCursorFg:   VGABlack,        // #000000 Black text on Green hover bar
			ConfirmBg:       VGARed,          // #A80000 Borland Red alert box
			ConfirmFg:       VGABrightWhite,
			ConfirmBorder:   VGABrightYellow,
			ConfirmCursorBg: VGABrightYellow, // #FFFF55 Yellow selection bar
			ConfirmCursorFg: VGABlack,
		}
	case ThemeGreenPhosphor:
		// Authentic IBM 5151 Monochrome Green Phosphor CRT monitor
		return ColorPalette{
			Name:            ThemeGreenPhosphor,
			PanelBg:         PhosphorBg,
			PanelFg:         PhosphorBright,
			SecondaryBg:     PhosphorBg,
			SecondaryFg:     PhosphorDim,
			SecondaryBorder: PhosphorDim,
			NavBg:           PhosphorBg,
			NavFg:           PhosphorBright,
			NavUnfocusedBg:  PhosphorDim,
			NavUnfocusedFg:  PhosphorBg,
			HeaderBg:        PhosphorBg,
			HeaderFg:        PhosphorBright,
			FooterBg:        PhosphorBg,
			FooterFg:        PhosphorBright,
			LogBg:           PhosphorBg,
			LogFg:           PhosphorDim,
			LogBorder:       PhosphorDim,
			ActiveBorder:    PhosphorGlow,
			ActiveTitle:     PhosphorGlow,
			InactiveBorder:  PhosphorDim,
			InactiveTitle:   PhosphorDim,
			CursorBg:        PhosphorBright,
			CursorFg:        PhosphorBg,
			ModalBg:         PhosphorModalBg,
			ModalFg:         PhosphorBright,
			ModalBorder:     PhosphorGlow,
			ModalTitle:      PhosphorGlow,
			ModalCursorBg:   PhosphorBright,
			ModalCursorFg:   PhosphorBg,
			ConfirmBg:       PhosphorModalBg,
			ConfirmFg:       PhosphorGlow,
			ConfirmBorder:   PhosphorGlow,
			ConfirmCursorBg: PhosphorGlow,
			ConfirmCursorFg: PhosphorBg,
		}
	case ThemeDark:
		return ColorPalette{
			Name:            ThemeDark,
			PanelBg:         VGABlack,
			PanelFg:         VGABrightWhite,
			SecondaryBg:     VGABlack,
			SecondaryFg:     VGABrightWhite,
			SecondaryBorder: VGADarkGray,
			NavBg:           VGABlack,
			NavFg:           VGABrightWhite,
			NavUnfocusedBg:  VGABlue,
			NavUnfocusedFg:  VGABrightWhite,
			HeaderBg:        VGABlack,
			HeaderFg:        VGABrightWhite,
			FooterBg:        VGABlack,
			FooterFg:        VGABrightWhite,
			LogBg:           VGABlack,
			LogFg:           VGABrightWhite,
			LogBorder:       VGADarkGray,
			ActiveBorder:    VGABrightCyan,
			ActiveTitle:     VGABrightCyan,
			InactiveBorder:  VGADarkGray,
			InactiveTitle:   VGABrightWhite,
			CursorBg:        VGABrightCyan,
			CursorFg:        VGABlack,
			ModalBg:         VGABlue,
			ModalFg:         VGABrightWhite,
			ModalBorder:     VGABrightCyan,
			ModalTitle:      VGABrightYellow,
			ModalCursorBg:   VGABrightCyan,
			ModalCursorFg:   VGABlack,
			ConfirmBg:       VGARed,
			ConfirmFg:       VGABrightWhite,
			ConfirmBorder:   VGABrightYellow,
			ConfirmCursorBg: VGABrightYellow,
			ConfirmCursorFg: VGABlack,
		}
	default:
		// Exact Norton Commander MS-DOS colors (from screenshot 2: Blue #0000A8 panels, Bright Cyan #55FFFF borders & files,
		// Yellow #FFFF55 column headers, Teal #00A8A8 cursor bar with Black text, and Silver #A8A8A8 dialog boxes with White double frame & Yellow cursor bar!)
		return ColorPalette{
			Name:            ThemeNorton,
			PanelBg:         VGABlue,         // #0000A8 Norton Commander panel blue
			PanelFg:         VGABrightCyan,   // #55FFFF Bright Cyan text inside blue panels
			SecondaryBg:     VGABlue,         // #0000A8 Blue secondary panels
			SecondaryFg:     VGABrightCyan,   // #55FFFF Bright Cyan text
			SecondaryBorder: VGABrightCyan,   // #55FFFF Bright Cyan border
			NavBg:           VGABlue,         // #0000A8 Blue left panel
			NavFg:           VGABrightCyan,   // #55FFFF Bright Cyan items
			NavUnfocusedBg:  VGACyan,         // #00A8A8 Teal bar on active module
			NavUnfocusedFg:  VGABlack,        // #000000 Black text on Teal bar
			HeaderBg:        VGABlue,         // #0000A8 Classic Norton Commander Blue top box with Teal title badge
			HeaderFg:        VGABrightCyan,   // #55FFFF
			FooterBg:        VGABlack,        // #000000 Bottom 1Help 2Menu bar background
			FooterFg:        VGABrightWhite,  // #FFFFFF
			LogBg:           VGABlack,        // #000000 MS-DOS C:\> output console at bottom
			LogFg:           VGASilver,       // #A8A8A8 MS-DOS console text
			LogBorder:       VGABrightCyan,   // #55FFFF
			ActiveBorder:    VGABrightCyan,   // #55FFFF Bright Cyan double border (with Teal title badge!)
			ActiveTitle:     VGABrightYellow, // #FFFF55
			InactiveBorder:  VGABrightCyan,   // #55FFFF Bright Cyan border on all panels
			InactiveTitle:   VGABrightCyan,   // #55FFFF
			CursorBg:        VGACyan,         // #00A8A8 Classic Norton Commander Teal cursor bar (config sys)
			CursorFg:        VGABlack,        // #000000 Black text on Teal cursor bar
			ModalBg:         VGASilver,       // #A8A8A8 Classic Norton Commander gray dialog box (Drive Letter)
			ModalFg:         VGABrightWhite,  // #FFFFFF Bright White text inside gray dialog box
			ModalBorder:     VGABrightWhite,  // #FFFFFF Double White border on gray dialog box
			ModalTitle:      VGABlack,        // #000000 Title on White badge
			ModalCursorBg:   VGABrightYellow, // #FFFF55 Bright Yellow selection bar inside dialog (button 'A')
			ModalCursorFg:   VGABlack,        // #000000 Bold Black text on Yellow bar
			ConfirmBg:       VGARed,          // #A80000 Classic Norton Commander Red warning dialog
			ConfirmFg:       VGABrightWhite,  // #FFFFFF
			ConfirmBorder:   VGABrightWhite,  // #FFFFFF
			ConfirmCursorBg: VGABrightYellow, // #FFFF55 Yellow selection bar
			ConfirmCursorFg: VGABlack,        // #000000
		}
	}
}

func ApplyColorTheme(theme ColorThemeName) ColorPalette {
	currentPalette = GetColorPalette(theme)
	tview.Styles.PrimitiveBackgroundColor = currentPalette.PanelBg
	tview.Styles.ContrastBackgroundColor = currentPalette.ModalBg
	tview.Styles.MoreContrastBackgroundColor = currentPalette.ConfirmBg
	tview.Styles.BorderColor = currentPalette.InactiveBorder
	tview.Styles.TitleColor = currentPalette.InactiveTitle
	tview.Styles.PrimaryTextColor = currentPalette.PanelFg
	return currentPalette
}

func NextColorTheme(curr ColorThemeName) ColorThemeName {
	switch curr {
	case ThemeNorton:
		return ThemeTurboC
	case ThemeTurboC:
		return ThemeGreenPhosphor
	case ThemeGreenPhosphor:
		return ThemeDark
	default:
		return ThemeNorton
	}
}

// HeaderCell creates a non-selectable table column header cell styled for the active theme.
func HeaderCell(text string) *tview.TableCell {
	col := VGABrightYellow
	if currentPalette.Name == ThemeGreenPhosphor {
		col = PhosphorGlow
	}
	return tview.NewTableCell(text).
		SetTextColor(col).
		SetAttributes(tcell.AttrBold).
		SetSelectable(false)
}

// KeyCell creates a bold primary-key table cell that cleanly inverts to CursorFg when selected.
func KeyCell(text string) *tview.TableCell {
	col := VGABrightWhite
	if currentPalette.Name == ThemeGreenPhosphor {
		col = PhosphorGlow
	}
	return tview.NewTableCell(text).
		SetTextColor(col).
		SetAttributes(tcell.AttrBold)
}

// AccentCell creates an accent table cell that cleanly inverts to CursorFg when selected.
func AccentCell(text string) *tview.TableCell {
	col := VGABrightCyan
	if currentPalette.Name == ThemeGreenPhosphor {
		col = PhosphorBright
	}
	return tview.NewTableCell(text).SetTextColor(col)
}

// MutedCell creates a muted/placeholder table cell that cleanly inverts to CursorFg when selected.
func MutedCell(text string) *tview.TableCell {
	col := VGASilver
	if currentPalette.Name == ThemeGreenPhosphor {
		col = PhosphorDim
	}
	return tview.NewTableCell(text).SetTextColor(col).SetSelectable(true)
}

// StatusCell creates a status table cell using tview cell color attributes so it cleanly inverts to CursorFg when selected.
func StatusCell(status string) *tview.TableCell {
	u := strings.ToUpper(status)
	if currentPalette.Name == ThemeGreenPhosphor {
		switch {
		case strings.Contains(u, "OFFLINE") || strings.Contains(u, "DOWN") ||
			strings.Contains(u, "NO READY") || strings.Contains(u, "SIN SERVICIOS") ||
			strings.Contains(u, "INVALID") || strings.Contains(u, "BLOCK") ||
			strings.Contains(u, "HIGH") || strings.Contains(u, "DRAINED") ||
			strings.Contains(u, "FAILED") || strings.Contains(u, "CRITICAL") ||
			strings.Contains(u, "ORA-"):
			return tview.NewTableCell(" " + status + " ").
				SetTextColor(PhosphorBg).
				SetBackgroundColor(PhosphorBright).
				SetAttributes(tcell.AttrBold)
		case strings.Contains(u, "READ ONLY") || strings.Contains(u, "INACTIVE") ||
			strings.Contains(u, "UNKNOWN") || strings.Contains(u, "PARCIAL") ||
			strings.Contains(u, "STALE") || strings.Contains(u, "MED") ||
			strings.Contains(u, "WARNING") || strings.Contains(u, "RUNNING"):
			return tview.NewTableCell(status).SetTextColor(PhosphorDim)
		default:
			return tview.NewTableCell(status).SetTextColor(PhosphorGlow).SetAttributes(tcell.AttrBold)
		}
	}

	switch {
	case strings.Contains(u, "OFFLINE") || strings.Contains(u, "DOWN") ||
		strings.Contains(u, "NO READY") || strings.Contains(u, "SIN SERVICIOS") ||
		strings.Contains(u, "INVALID") || strings.Contains(u, "BLOCK") ||
		strings.Contains(u, "HIGH") || strings.Contains(u, "DRAINED") ||
		strings.Contains(u, "FAILED") || strings.Contains(u, "CRITICAL") ||
		strings.Contains(u, "ORA-"):
		return tview.NewTableCell(status).SetTextColor(VGABrightRed).SetAttributes(tcell.AttrBold)
	case strings.Contains(u, "READ ONLY") || strings.Contains(u, "INACTIVE") ||
		strings.Contains(u, "UNKNOWN") || strings.Contains(u, "PARCIAL") ||
		strings.Contains(u, "STALE") || strings.Contains(u, "MED") ||
		strings.Contains(u, "WARNING") || strings.Contains(u, "RUNNING"):
		return tview.NewTableCell(status).SetTextColor(VGABrightYellow).SetAttributes(tcell.AttrBold)
	case strings.Contains(u, "ONLINE") || strings.Contains(u, "OPEN") ||
		strings.Contains(u, "READ WRITE") || strings.Contains(u, "CONNECTED") ||
		strings.Contains(u, "COMPLETED") || strings.Contains(u, "READY") ||
		strings.Contains(u, "APPLYING_LOG") ||
		(strings.Contains(u, "MOUNTED") && !strings.Contains(u, "DISMOUNTED")) ||
		strings.Contains(u, "FRESH") || strings.Contains(u, "ACTIVE") ||
		strings.Contains(u, "LOW") || u == "YES" || u == "INFO" || strings.HasPrefix(u, "+00"):
		return tview.NewTableCell(status).SetTextColor(VGABrightGreen).SetAttributes(tcell.AttrBold)
	default:
		return tview.NewTableCell(status).SetTextColor(currentPalette.PanelFg)
	}
}

// BarCell creates a progress bar table cell whose color cleanly inverts to CursorFg when the row is selected.
func BarCell(pct float64, width int) *tview.TableCell {
	txt := ASCIIBar(pct, width, false)
	if currentPalette.Name == ThemeGreenPhosphor {
		if pct >= 90.0 {
			return tview.NewTableCell(txt).SetTextColor(PhosphorGlow).SetAttributes(tcell.AttrBold)
		}
		return tview.NewTableCell(txt).SetTextColor(PhosphorBright).SetAttributes(tcell.AttrBold)
	}
	col := VGABrightGreen
	if pct >= 90.0 {
		col = VGABrightRed
	} else if pct >= 80.0 {
		col = VGABrightYellow
	}
	return tview.NewTableCell(txt).SetTextColor(col).SetAttributes(tcell.AttrBold)
}

// ApplyBorderTheme configures tview's box-drawing characters according to the selected style:
// - BorderMixedUnicode (default): Simple border (┌ ─ ┐ │ └ ┘) for windows + Double border (╔ ═ ╗ ║ ╚ ╝) for focused windows & popup menus.
// - BorderDoubleUnicode: Double border (╔ ═ ╗ ║ ╚ ╝) for all windows.
// - BorderSingleUnicode: Simple border (┌ ─ ┐ │ └ ┘) for all windows.
// - BorderClassicASCII: 7-bit ASCII (+ - = |) fallback.
func ApplyBorderTheme(style model.BorderStyle) {
	currentBorderStyle = style
	switch style {
	case model.BorderDoubleUnicode:
		tview.Borders.Horizontal = '═'
		tview.Borders.Vertical = '║'
		tview.Borders.TopLeft = '╔'
		tview.Borders.TopRight = '╗'
		tview.Borders.BottomLeft = '╚'
		tview.Borders.BottomRight = '╝'
		tview.Borders.LeftT = '╠'
		tview.Borders.RightT = '╣'
		tview.Borders.TopT = '╦'
		tview.Borders.BottomT = '╩'
		tview.Borders.Cross = '╬'

		tview.Borders.HorizontalFocus = '═'
		tview.Borders.VerticalFocus = '║'
		tview.Borders.TopLeftFocus = '╔'
		tview.Borders.TopRightFocus = '╗'
		tview.Borders.BottomLeftFocus = '╚'
		tview.Borders.BottomRightFocus = '╝'

	case model.BorderSingleUnicode:
		tview.Borders.Horizontal = '─'
		tview.Borders.Vertical = '│'
		tview.Borders.TopLeft = '┌'
		tview.Borders.TopRight = '┐'
		tview.Borders.BottomLeft = '└'
		tview.Borders.BottomRight = '┘'
		tview.Borders.LeftT = '├'
		tview.Borders.RightT = '┤'
		tview.Borders.TopT = '┬'
		tview.Borders.BottomT = '┴'
		tview.Borders.Cross = '┼'

		tview.Borders.HorizontalFocus = '─'
		tview.Borders.VerticalFocus = '│'
		tview.Borders.TopLeftFocus = '┌'
		tview.Borders.TopRightFocus = '┐'
		tview.Borders.BottomLeftFocus = '└'
		tview.Borders.BottomRightFocus = '┘'

	case model.BorderClassicASCII:
		tview.Borders.Horizontal = '-'
		tview.Borders.Vertical = '|'
		tview.Borders.TopLeft = '+'
		tview.Borders.TopRight = '+'
		tview.Borders.BottomLeft = '+'
		tview.Borders.BottomRight = '+'
		tview.Borders.LeftT = '+'
		tview.Borders.RightT = '+'
		tview.Borders.TopT = '+'
		tview.Borders.BottomT = '+'
		tview.Borders.Cross = '+'

		tview.Borders.HorizontalFocus = '='
		tview.Borders.VerticalFocus = '|'
		tview.Borders.TopLeftFocus = '+'
		tview.Borders.TopRightFocus = '+'
		tview.Borders.BottomLeftFocus = '+'
		tview.Borders.BottomRightFocus = '+'

	default: // BorderMixedUnicode: Simple (┌─┐│└┘) + Double on Focus/Modals (╔═╗║╚╝)
		tview.Borders.Horizontal = '─'
		tview.Borders.Vertical = '│'
		tview.Borders.TopLeft = '┌'
		tview.Borders.TopRight = '┐'
		tview.Borders.BottomLeft = '└'
		tview.Borders.BottomRight = '┘'
		tview.Borders.LeftT = '├'
		tview.Borders.RightT = '┤'
		tview.Borders.TopT = '┬'
		tview.Borders.BottomT = '┴'
		tview.Borders.Cross = '┼'

		tview.Borders.HorizontalFocus = '═'
		tview.Borders.VerticalFocus = '║'
		tview.Borders.TopLeftFocus = '╔'
		tview.Borders.TopRightFocus = '╗'
		tview.Borders.BottomLeftFocus = '╚'
		tview.Borders.BottomRightFocus = '╝'
	}
}

// NextBorderStyle cycles through the 4 available window frame styles.
func NextBorderStyle(curr model.BorderStyle) model.BorderStyle {
	switch curr {
	case model.BorderMixedUnicode:
		return model.BorderDoubleUnicode
	case model.BorderDoubleUnicode:
		return model.BorderSingleUnicode
	case model.BorderSingleUnicode:
		return model.BorderClassicASCII
	default:
		return model.BorderMixedUnicode
	}
}

// ASCIIBar returns a visual progress bar ([████████░░] 84.0% in Unicode mode, or [########--] in ASCII mode).
func ASCIIBar(pct float64, width int, colorize bool) string {
	if width <= 0 {
		width = 10
	}
	filled := int((pct / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	fillChar := "█"
	emptyChar := "░"
	if currentBorderStyle == model.BorderClassicASCII {
		fillChar = "#"
		emptyChar = "-"
	}
	bar := "[" + strings.Repeat(fillChar, filled) + strings.Repeat(emptyChar, width-filled) + "]"
	if !colorize {
		return fmt.Sprintf("%s %5.1f%%", bar, pct)
	}
	if currentPalette.Name == ThemeGreenPhosphor {
		if pct >= 90.0 {
			return fmt.Sprintf("[#001000:#33ff33:b]%s %5.1f%%[-:-:-]", bar, pct)
		}
		return fmt.Sprintf("[#33ff33::b]%s %5.1f%%[-::-]", bar, pct)
	}
	color := "[#55ff55::b]"
	if pct >= 90.0 {
		color = "[#ff5555::b]"
	} else if pct >= 80.0 {
		color = "[#ffff55::b]"
	}
	return fmt.Sprintf("%s%s %5.1f%%[-::-]", color, bar, pct)
}

// ColorStatus formats status strings with high-contrast colors adapted to Norton Commander, Turbo C, Green Phosphor, or Dark.
func ColorStatus(status string) string {
	u := strings.ToUpper(status)
	if currentPalette.Name == ThemeGreenPhosphor {
		switch {
		case strings.Contains(u, "OFFLINE") || strings.Contains(u, "DOWN") ||
			strings.Contains(u, "NO READY") || strings.Contains(u, "SIN SERVICIOS") ||
			strings.Contains(u, "INVALID") || strings.Contains(u, "BLOCK") ||
			strings.Contains(u, "HIGH") || strings.Contains(u, "DRAINED") ||
			strings.Contains(u, "FAILED") || strings.Contains(u, "CRITICAL") ||
			strings.Contains(u, "ORA-"):
			return "[#001000:#33ff33:b] " + status + " [-:-:-]"
		case strings.Contains(u, "READ ONLY") || strings.Contains(u, "INACTIVE") ||
			strings.Contains(u, "UNKNOWN") || strings.Contains(u, "PARCIAL") ||
			strings.Contains(u, "STALE") || strings.Contains(u, "MED") ||
			strings.Contains(u, "WARNING") || strings.Contains(u, "RUNNING"):
			return "[#99ff99]" + status + "[-]"
		default:
			return "[#33ff33::b]" + status + "[-::-]"
		}
	}

	switch {
	case strings.Contains(u, "OFFLINE") || strings.Contains(u, "DOWN") ||
		strings.Contains(u, "NO READY") || strings.Contains(u, "SIN SERVICIOS") ||
		strings.Contains(u, "INVALID") || strings.Contains(u, "BLOCK") ||
		strings.Contains(u, "HIGH") || strings.Contains(u, "DRAINED") ||
		strings.Contains(u, "FAILED") || strings.Contains(u, "CRITICAL") ||
		strings.Contains(u, "ORA-"):
		return "[#ff5555::b]" + status + "[-::-]"
	case strings.Contains(u, "READ ONLY") || strings.Contains(u, "INACTIVE") ||
		strings.Contains(u, "UNKNOWN") || strings.Contains(u, "PARCIAL") ||
		strings.Contains(u, "STALE") || strings.Contains(u, "MED") ||
		strings.Contains(u, "WARNING") || strings.Contains(u, "RUNNING"):
		return "[#ffff55::b]" + status + "[-::-]"
	case strings.Contains(u, "ONLINE") || strings.Contains(u, "OPEN") ||
		strings.Contains(u, "READ WRITE") || strings.Contains(u, "CONNECTED") ||
		strings.Contains(u, "COMPLETED") || strings.Contains(u, "READY") ||
		strings.Contains(u, "APPLYING_LOG") ||
		(strings.Contains(u, "MOUNTED") && !strings.Contains(u, "DISMOUNTED")) ||
		strings.Contains(u, "FRESH") || strings.Contains(u, "ACTIVE") ||
		strings.Contains(u, "LOW") || u == "YES" || u == "INFO":
		return "[#55ff55::b]" + status + "[-::-]"
	default:
		return status
	}
}

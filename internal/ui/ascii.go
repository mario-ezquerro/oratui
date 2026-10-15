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
	// Authentic MS-DOS VGA 16-color palette aligned to both 24-bit TrueColor AND XTerm 256-color cubes
	// so colors render identically on local terminals and over SSH (xterm-256color):
	VGABlack        = tcell.NewRGBColor(0, 0, 0)       // #000000 (XTerm 16)
	VGABlue         = tcell.NewRGBColor(0, 0, 175)     // #0000AF (XTerm 19 - Norton Commander & Borland Turbo C++ Royal Blue)
	VGAGreen        = tcell.NewRGBColor(0, 175, 0)     // #00AF00 (XTerm 34 - Borland Turbo C++ selection bar green)
	VGACyan         = tcell.NewRGBColor(0, 175, 175)   // #00AFAF (XTerm 37 - Norton Commander cursor bar & Turbo C++ bottom Project window)
	VGARed          = tcell.NewRGBColor(175, 0, 0)     // #AF0000 (XTerm 124 - Borland Turbo C++ red shortcut letters)
	VGASilver       = tcell.NewRGBColor(168, 168, 168) // #A8A8A8 (XTerm 248 - Norton Commander dialog box & Turbo C++ menu bar gray)
	VGADarkGray     = tcell.NewRGBColor(95, 95, 95)    // #5F5F5F (XTerm 59)
	VGABrightBlue   = tcell.NewRGBColor(95, 95, 255)   // #5F5FFF (XTerm 63)
	VGABrightGreen  = tcell.NewRGBColor(95, 255, 95)   // #5FFF5F (XTerm 83 - Turbo C++ [■] and [↑] window icons)
	VGABrightCyan   = tcell.NewRGBColor(95, 255, 255)  // #5FFFFF (XTerm 87 - Norton Commander double borders & file list text)
	VGABrightRed    = tcell.NewRGBColor(255, 95, 95)   // #FF5F5F (XTerm 203)
	VGABrightYellow = tcell.NewRGBColor(255, 255, 95)  // #FFFF5F (XTerm 227 - Norton Commander column headers & Turbo C++ window text)
	VGABrightWhite  = tcell.NewRGBColor(255, 255, 255) // #FFFFFF (XTerm 231 - Norton Commander dialog borders & Turbo C++ double frame)

	// Authentic IBM 5151 / VT100 P1 Green Phosphor CRT palette
	PhosphorBg      = tcell.NewRGBColor(0, 16, 0)      // #001000 Deep CRT screen black-green
	PhosphorPanelBg = tcell.NewRGBColor(0, 24, 0)      // #001800 Subtle CRT scanline dark green
	PhosphorModalBg = tcell.NewRGBColor(0, 42, 0)      // #002A00 Dialog box dark green
	PhosphorDim     = tcell.NewRGBColor(0, 175, 0)     // #00AF00 Normal phosphor green (XTerm 34)
	PhosphorBright  = tcell.NewRGBColor(0, 255, 95)    // #00FF5F High-intensity phosphor green (XTerm 47)
	PhosphorGlow    = tcell.NewRGBColor(135, 255, 135) // #87FF87 Peak phosphor bloom (XTerm 120)
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
		// Exact Borland Turbo C++ 3.0 MS-DOS colors (from Screenshot 1):
		// - Top menu bar & bottom status bar: Silver (#A8A8A8) with Black (#000000) text and Red (#AF0000) shortcut letters.
		// - Left & main windows: Royal Blue (#0000AF) with Double Bright White (#FFFFFF) borders, Green [■]/[↑] icons,
		//   Bright Yellow (#FFFF5F) text, and Green (#00AF00) selection bar with Black (#000000) text.
		// - Bottom "Project:" window & secondary bottom panel: Teal/Cyan (#00AFAF) background with Single Dark Blue (#0000AF) border and Dark Blue text.
		// - Floating dropdown menus: Silver (#A8A8A8) with Single Black (#000000) border, Black text, Red (#AF0000) hotkeys, Green (#00AF00) selection bar, and Black drop shadow.
		return ColorPalette{
			Name:            ThemeTurboC,
			PanelBg:         VGABlue,         // #0000AF Royal Blue window background
			PanelFg:         VGABrightYellow, // #FFFF5F Bright Yellow text inside blue windows
			SecondaryBg:     VGACyan,         // #00AFAF Teal/Cyan bottom Project window background
			SecondaryFg:     VGABlue,         // #0000AF Dark Blue text on Cyan window
			SecondaryBorder: VGABlue,         // #0000AF Dark Blue single border on Cyan window
			NavBg:           VGABlue,         // #0000AF Royal Blue left window (like Turbo C editor window)
			NavFg:           VGABrightYellow, // #FFFF5F Bright Yellow text
			NavUnfocusedBg:  VGAGreen,        // #00AF00 Green active module bar
			NavUnfocusedFg:  VGABlack,        // #000000 Black text on Green bar
			HeaderBg:        VGASilver,       // #A8A8A8 Silver top menu bar (≡ File Edit Search Run...)
			HeaderFg:        VGABlack,        // #000000 Black text on top menu bar
			FooterBg:        VGASilver,       // #A8A8A8 Silver bottom F1 Help bar
			FooterFg:        VGABlack,        // #000000 Black text on bottom bar
			LogBg:           VGACyan,         // #00AFAF Bottom "Project:" Cyan panel
			LogFg:           VGABlue,         // #0000AF Dark Blue text inside Cyan panel
			LogBorder:       VGABlue,         // #0000AF Single Dark Blue border on Cyan panel
			ActiveBorder:    VGABrightWhite,  // #FFFFFF Double White border on Blue windows
			ActiveTitle:     VGABrightWhite,  // #FFFFFF White title with [#5fff5f][■] and [↑]
			InactiveBorder:  VGABrightWhite,  // #FFFFFF Double White border on Blue windows
			InactiveTitle:   VGABrightWhite,
			CursorBg:        VGAGreen,        // #00AF00 Classic Turbo C Green selection bar
			CursorFg:        VGABlack,        // #000000 Black text on Green selection bar
			ModalBg:         VGASilver,       // #A8A8A8 Classic Borland Silver dropdown box
			ModalFg:         VGABlack,        // #000000 Black text in dropdown
			ModalBorder:     VGABlack,        // #000000 Single Black border inside Silver rim
			ModalTitle:      VGABlack,        // #000000 Title in Black on Silver
			ModalCursorBg:   VGAGreen,        // #00AF00 Classic Turbo C Green hover bar
			ModalCursorFg:   VGABlack,        // #000000 Black text on Green hover bar
			ConfirmBg:       VGARed,          // #AF0000 Borland Red alert box
			ConfirmFg:       VGABrightWhite,
			ConfirmBorder:   VGABrightYellow,
			ConfirmCursorBg: VGABrightYellow, // #FFFF5F Yellow selection bar
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
		// Exact Norton Commander MS-DOS colors (from Screenshot 2):
		// - All panels: Royal Blue (#0000AF) background with Double Bright Cyan (#5FFFFF) borders.
		// - Column headers: Bright Yellow (#FFFF5F).
		// - Data rows: Uniform Bright Cyan (#5FFFFF) text on Royal Blue (#0000AF).
		// - Active panel title & cursor bar: Teal/Cyan (#00AFAF) background with Black (#000000) text.
		// - Popup dialog box ("Drive Letter"): Silver (#A8A8A8) with 1-char outer rim, Double Bright White (#FFFFFF) border,
		//   solid Bright White title badge with dark text, Bright White item text, Bright Yellow (#FFFF5F) selection bar with Black text, and Black drop shadow.
		// - Bottom DOS console & 1Help..10Quit bar: Black (#000000) background with Silver C:\> prompt and Teal (#00AFAF) key blocks.
		return ColorPalette{
			Name:            ThemeNorton,
			PanelBg:         VGABlue,         // #0000AF Norton Commander Royal Blue
			PanelFg:         VGABrightCyan,   // #5FFFFF Bright Cyan text inside blue panels
			SecondaryBg:     VGABlue,         // #0000AF Royal Blue secondary panels
			SecondaryFg:     VGABrightCyan,   // #5FFFFF Bright Cyan text
			SecondaryBorder: VGABrightCyan,   // #5FFFFF Double Bright Cyan border
			NavBg:           VGABlue,         // #0000AF Royal Blue left panel
			NavFg:           VGABrightCyan,   // #5FFFFF Bright Cyan items
			NavUnfocusedBg:  VGACyan,         // #00AFAF Teal bar on active module
			NavUnfocusedFg:  VGABlack,        // #000000 Black text on Teal bar
			HeaderBg:        VGABlue,         // #0000AF Classic Norton Commander Blue top bar
			HeaderFg:        VGABrightCyan,   // #5FFFFF
			FooterBg:        VGABlack,        // #000000 Bottom 1Help 2Menu bar background
			FooterFg:        VGABrightWhite,  // #FFFFFF
			LogBg:           VGABlack,        // #000000 MS-DOS C:\> output console at bottom
			LogFg:           VGASilver,       // #A8A8A8 MS-DOS console text
			LogBorder:       VGABrightCyan,   // #5FFFFF
			ActiveBorder:    VGABrightCyan,   // #5FFFFF Double Bright Cyan border (with Teal title badge)
			ActiveTitle:     VGABrightYellow, // #FFFF5F
			InactiveBorder:  VGABrightCyan,   // #5FFFFF Double Bright Cyan border on all panels
			InactiveTitle:   VGABrightCyan,   // #5FFFFF
			CursorBg:        VGACyan,         // #00AFAF Classic Norton Commander Teal cursor bar
			CursorFg:        VGABlack,        // #000000 Black text on Teal cursor bar
			ModalBg:         VGASilver,       // #A8A8A8 Classic Norton Commander gray dialog box (Drive Letter)
			ModalFg:         VGABrightWhite,  // #FFFFFF Bright White text inside gray dialog box
			ModalBorder:     VGABrightWhite,  // #FFFFFF Double White border on gray dialog box
			ModalTitle:      VGABlack,        // #000000 Title on White badge
			ModalCursorBg:   VGABrightYellow, // #FFFF5F Bright Yellow selection bar inside dialog (button 'A')
			ModalCursorFg:   VGABlack,        // #000000 Bold Black text on Yellow bar
			ConfirmBg:       VGARed,          // #AF0000 Classic Norton Commander Red warning dialog
			ConfirmFg:       VGABrightWhite,  // #FFFFFF
			ConfirmBorder:   VGABrightWhite,  // #FFFFFF
			ConfirmCursorBg: VGABrightYellow, // #FFFF5F Yellow selection bar
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

// HeaderCell creates a non-selectable table column header cell styled for the active theme:
// - Norton Commander: Bright Yellow (#FFFF5F) like "C:↓ Name   Name   Name" in Screenshot 2.
// - Borland Turbo C++: Bright White (#FFFFFF) on Blue panels.
// - Green Phosphor: Peak Phosphor Glow (#87FF87).
func HeaderCell(text string) *tview.TableCell {
	col := VGABrightYellow
	switch currentPalette.Name {
	case ThemeTurboC:
		col = VGABrightWhite
	case ThemeGreenPhosphor:
		col = PhosphorGlow
	}
	return tview.NewTableCell(text).
		SetTextColor(col).
		SetAttributes(tcell.AttrBold).
		SetSelectable(false)
}

// KeyCell creates a primary-key table cell that matches the authentic uniform row color of each theme:
// - Norton Commander: Bright Cyan (#5FFFFF) like "ADINF", "DOS", "GAMES" in Screenshot 2.
// - Borland Turbo C++: Bright Yellow (#FFFF5F) like the yellow code in Screenshot 1.
// - Green Phosphor: Peak Phosphor Glow (#87FF87).
func KeyCell(text string) *tview.TableCell {
	col := currentPalette.PanelFg
	switch currentPalette.Name {
	case ThemeNorton:
		col = VGABrightCyan
	case ThemeTurboC:
		col = VGABrightYellow
	case ThemeGreenPhosphor:
		col = PhosphorGlow
	case ThemeDark:
		col = VGABrightWhite
	}
	return tview.NewTableCell(text).
		SetTextColor(col).
		SetAttributes(tcell.AttrBold)
}

// AccentCell creates an accent table cell that cleanly inverts to CursorFg when selected.
func AccentCell(text string) *tview.TableCell {
	col := currentPalette.PanelFg
	switch currentPalette.Name {
	case ThemeNorton:
		col = VGABrightCyan
	case ThemeTurboC:
		col = VGABrightYellow
	case ThemeGreenPhosphor:
		col = PhosphorBright
	default:
		col = VGABrightCyan
	}
	return tview.NewTableCell(text).SetTextColor(col)
}

// MutedCell creates a muted/placeholder table cell that cleanly inverts to CursorFg when selected.
func MutedCell(text string) *tview.TableCell {
	col := VGASilver
	switch currentPalette.Name {
	case ThemeNorton:
		col = VGABrightCyan
	case ThemeTurboC:
		col = VGABrightYellow
	case ThemeGreenPhosphor:
		col = PhosphorDim
	}
	return tview.NewTableCell(text).SetTextColor(col).SetSelectable(true)
}

// StatusCell creates a status table cell that keeps Norton Commander (Bright Cyan) and Turbo C++ (Bright Yellow)
// panels clean and authentic, while highlighting warnings/errors in Yellow/Red.
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
			return tview.NewTableCell(status).SetTextColor(PhosphorBright).SetAttributes(tcell.AttrBold)
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
	default:
		// Normal/healthy statuses use the theme's authentic row foreground:
		// - Norton Commander: Bright Cyan (#5FFFFF)
		// - Borland Turbo C++: Bright Yellow (#FFFF5F)
		// - Terminal Oscuro: Bright Green (#5FFF5F)
		if currentPalette.Name == ThemeDark {
			return tview.NewTableCell(status).SetTextColor(VGABrightGreen).SetAttributes(tcell.AttrBold)
		}
		return tview.NewTableCell(status).SetTextColor(currentPalette.PanelFg)
	}
}

// BarCell creates a progress bar table cell whose color matches the active retro theme.
func BarCell(pct float64, width int) *tview.TableCell {
	txt := ASCIIBar(pct, width, false)
	if currentPalette.Name == ThemeGreenPhosphor {
		if pct >= 90.0 {
			return tview.NewTableCell(txt).SetTextColor(PhosphorGlow).SetAttributes(tcell.AttrBold)
		}
		return tview.NewTableCell(txt).SetTextColor(PhosphorBright)
	}
	col := currentPalette.PanelFg
	if currentPalette.Name == ThemeDark {
		col = VGABrightGreen
	}
	if pct >= 90.0 {
		col = VGABrightRed
	} else if pct >= 80.0 {
		col = VGABrightYellow
	}
	return tview.NewTableCell(txt).SetTextColor(col)
}

// ApplyBorderTheme configures tview's box-drawing characters according to the selected style.
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

	default: // BorderMixedUnicode
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
			return fmt.Sprintf("[#001000:#00ff5f:b]%s %5.1f%%[-:-:-]", bar, pct)
		}
		return fmt.Sprintf("[#00ff5f::b]%s %5.1f%%[-::-]", bar, pct)
	}
	color := "[#5fff5f::b]"
	if pct >= 90.0 {
		color = "[#ff5f5f::b]"
	} else if pct >= 80.0 {
		color = "[#ffff5f::b]"
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
			return "[#001000:#00ff5f:b] " + status + " [-:-:-]"
		case strings.Contains(u, "READ ONLY") || strings.Contains(u, "INACTIVE") ||
			strings.Contains(u, "UNKNOWN") || strings.Contains(u, "PARCIAL") ||
			strings.Contains(u, "STALE") || strings.Contains(u, "MED") ||
			strings.Contains(u, "WARNING") || strings.Contains(u, "RUNNING"):
			return "[#87ff87]" + status + "[-]"
		default:
			return "[#00ff5f::b]" + status + "[-::-]"
		}
	}

	switch {
	case strings.Contains(u, "OFFLINE") || strings.Contains(u, "DOWN") ||
		strings.Contains(u, "NO READY") || strings.Contains(u, "SIN SERVICIOS") ||
		strings.Contains(u, "INVALID") || strings.Contains(u, "BLOCK") ||
		strings.Contains(u, "HIGH") || strings.Contains(u, "DRAINED") ||
		strings.Contains(u, "FAILED") || strings.Contains(u, "CRITICAL") ||
		strings.Contains(u, "ORA-"):
		return "[#ff5f5f::b]" + status + "[-::-]"
	case strings.Contains(u, "READ ONLY") || strings.Contains(u, "INACTIVE") ||
		strings.Contains(u, "UNKNOWN") || strings.Contains(u, "PARCIAL") ||
		strings.Contains(u, "STALE") || strings.Contains(u, "MED") ||
		strings.Contains(u, "WARNING") || strings.Contains(u, "RUNNING"):
		return "[#ffff5f::b]" + status + "[-::-]"
	case strings.Contains(u, "ONLINE") || strings.Contains(u, "OPEN") ||
		strings.Contains(u, "READ WRITE") || strings.Contains(u, "CONNECTED") ||
		strings.Contains(u, "COMPLETED") || strings.Contains(u, "READY") ||
		strings.Contains(u, "APPLYING_LOG") ||
		(strings.Contains(u, "MOUNTED") && !strings.Contains(u, "DISMOUNTED")) ||
		strings.Contains(u, "FRESH") || strings.Contains(u, "ACTIVE") ||
		strings.Contains(u, "LOW") || u == "YES" || u == "INFO":
		return "[#5fff5f::b]" + status + "[-::-]"
	default:
		return status
	}
}

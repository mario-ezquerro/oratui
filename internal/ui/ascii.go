package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mario/ora-tui/internal/model"
	"github.com/rivo/tview"
)

var currentBorderStyle = model.BorderMixedUnicode

type ColorThemeName string

const (
	ThemeNorton ColorThemeName = "NORTON COMMANDER"
	ThemeTurboC ColorThemeName = "BORLAND TURBO C"
	ThemeDark   ColorThemeName = "TERMINAL OSCURO"
)

type ColorPalette struct {
	Name            ColorThemeName
	PanelBg         tcell.Color
	PanelFg         tcell.Color
	NavBg           tcell.Color
	NavFg           tcell.Color
	NavUnfocusedBg  tcell.Color
	NavUnfocusedFg  tcell.Color
	HeaderBg        tcell.Color
	FooterBg        tcell.Color
	LogBg           tcell.Color
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
		return ColorPalette{
			Name:            ThemeTurboC,
			PanelBg:         tcell.ColorNavy,
			PanelFg:         tcell.ColorWhite,
			NavBg:           tcell.ColorSilver,
			NavFg:           tcell.ColorBlack,
			NavUnfocusedBg:  tcell.ColorTeal,
			NavUnfocusedFg:  tcell.ColorYellow,
			HeaderBg:        tcell.ColorTeal,
			FooterBg:        tcell.ColorTeal,
			LogBg:           tcell.ColorTeal,
			ActiveBorder:    tcell.ColorYellow,
			ActiveTitle:     tcell.ColorYellow,
			InactiveBorder:  tcell.ColorSilver,
			InactiveTitle:   tcell.ColorWhite,
			CursorBg:        tcell.ColorYellow,
			CursorFg:        tcell.ColorBlack,
			ModalBg:         tcell.ColorSilver,
			ModalFg:         tcell.ColorBlack,
			ModalBorder:     tcell.ColorWhite,
			ModalTitle:      tcell.ColorMaroon,
			ModalCursorBg:   tcell.ColorTeal,
			ModalCursorFg:   tcell.ColorYellow,
			ConfirmBg:       tcell.ColorMaroon,
			ConfirmFg:       tcell.ColorWhite,
			ConfirmBorder:   tcell.ColorYellow,
			ConfirmCursorBg: tcell.ColorYellow,
			ConfirmCursorFg: tcell.ColorBlack,
		}
	case ThemeDark:
		return ColorPalette{
			Name:            ThemeDark,
			PanelBg:         tcell.ColorBlack,
			PanelFg:         tcell.ColorWhite,
			NavBg:           tcell.ColorBlack,
			NavFg:           tcell.ColorWhite,
			NavUnfocusedBg:  tcell.ColorNavy,
			NavUnfocusedFg:  tcell.ColorWhite,
			HeaderBg:        tcell.ColorBlack,
			FooterBg:        tcell.ColorBlack,
			LogBg:           tcell.ColorBlack,
			ActiveBorder:    tcell.ColorAqua,
			ActiveTitle:     tcell.ColorAqua,
			InactiveBorder:  tcell.ColorGray,
			InactiveTitle:   tcell.ColorWhite,
			CursorBg:        tcell.ColorAqua,
			CursorFg:        tcell.ColorBlack,
			ModalBg:         tcell.ColorNavy,
			ModalFg:         tcell.ColorWhite,
			ModalBorder:     tcell.ColorAqua,
			ModalTitle:      tcell.ColorYellow,
			ModalCursorBg:   tcell.ColorAqua,
			ModalCursorFg:   tcell.ColorBlack,
			ConfirmBg:       tcell.ColorMaroon,
			ConfirmFg:       tcell.ColorWhite,
			ConfirmBorder:   tcell.ColorYellow,
			ConfirmCursorBg: tcell.ColorYellow,
			ConfirmCursorFg: tcell.ColorBlack,
		}
	default: // ThemeNorton (Classic MS-DOS Norton Commander / Turbo C Blue)
		return ColorPalette{
			Name:            ThemeNorton,
			PanelBg:         tcell.ColorNavy,
			PanelFg:         tcell.ColorWhite,
			NavBg:           tcell.ColorNavy,
			NavFg:           tcell.ColorWhite,
			NavUnfocusedBg:  tcell.ColorTeal,
			NavUnfocusedFg:  tcell.ColorYellow,
			HeaderBg:        tcell.ColorTeal,
			FooterBg:        tcell.ColorTeal,
			LogBg:           tcell.ColorBlack,
			ActiveBorder:    tcell.ColorYellow,
			ActiveTitle:     tcell.ColorYellow,
			InactiveBorder:  tcell.ColorAqua,
			InactiveTitle:   tcell.ColorAqua,
			CursorBg:        tcell.ColorAqua,
			CursorFg:        tcell.ColorBlack,
			ModalBg:         tcell.ColorTeal,
			ModalFg:         tcell.ColorWhite,
			ModalBorder:     tcell.ColorYellow,
			ModalTitle:      tcell.ColorYellow,
			ModalCursorBg:   tcell.ColorYellow,
			ModalCursorFg:   tcell.ColorBlack,
			ConfirmBg:       tcell.ColorMaroon,
			ConfirmFg:       tcell.ColorWhite,
			ConfirmBorder:   tcell.ColorYellow,
			ConfirmCursorBg: tcell.ColorYellow,
			ConfirmCursorFg: tcell.ColorBlack,
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
		return ThemeDark
	default:
		return ThemeNorton
	}
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
	color := "[lime::b]"
	if pct >= 90.0 {
		color = "[#ff5555::b]"
	} else if pct >= 80.0 {
		color = "[yellow::b]"
	}
	return fmt.Sprintf("%s%s %5.1f%%[-::-]", color, bar, pct)
}

// ColorStatus formats status strings with high-contrast colors suitable for both Navy Blue and Dark backgrounds.
func ColorStatus(status string) string {
	u := strings.ToUpper(status)
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
		return "[yellow::b]" + status + "[-::-]"
	case strings.Contains(u, "ONLINE") || strings.Contains(u, "OPEN") ||
		strings.Contains(u, "READ WRITE") || strings.Contains(u, "CONNECTED") ||
		strings.Contains(u, "COMPLETED") || strings.Contains(u, "READY") ||
		strings.Contains(u, "APPLYING_LOG") ||
		(strings.Contains(u, "MOUNTED") && !strings.Contains(u, "DISMOUNTED")) ||
		strings.Contains(u, "FRESH") || strings.Contains(u, "ACTIVE") ||
		strings.Contains(u, "LOW") || u == "YES" || u == "INFO":
		return "[lime::b]" + status + "[-::-]"
	default:
		return status
	}
}

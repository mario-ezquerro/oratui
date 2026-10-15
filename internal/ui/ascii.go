package ui

import (
	"fmt"
	"strings"

	"github.com/mario/ora-tui/internal/model"
	"github.com/rivo/tview"
)

var currentBorderStyle = model.BorderMixedUnicode

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
	color := "[green]"
	if pct >= 90.0 {
		color = "[red]"
	} else if pct >= 80.0 {
		color = "[yellow]"
	}
	return fmt.Sprintf("%s%s %5.1f%%[-]", color, bar, pct)
}

// ColorStatus formats status strings with tview colors.
func ColorStatus(status string) string {
	u := strings.ToUpper(status)
	switch {
	case strings.Contains(u, "ONLINE") || strings.Contains(u, "OPEN") ||
		strings.Contains(u, "READ WRITE") || strings.Contains(u, "CONNECTED") ||
		strings.Contains(u, "COMPLETED") || strings.Contains(u, "READY") ||
		strings.Contains(u, "APPLYING_LOG") ||
		(strings.Contains(u, "MOUNTED") && !strings.Contains(u, "DISMOUNTED")) ||
		strings.Contains(u, "FRESH") || strings.Contains(u, "ACTIVE") ||
		strings.Contains(u, "LOW") || u == "YES" || u == "INFO":
		return "[green]" + status + "[-]"
	case strings.Contains(u, "READ ONLY") || strings.Contains(u, "INACTIVE") ||
		strings.Contains(u, "STALE") || strings.Contains(u, "MED") ||
		strings.Contains(u, "WARNING") || strings.Contains(u, "RUNNING"):
		return "[yellow]" + status + "[-]"
	case strings.Contains(u, "OFFLINE") || strings.Contains(u, "DOWN") ||
		strings.Contains(u, "INVALID") || strings.Contains(u, "BLOCK") ||
		strings.Contains(u, "HIGH") || strings.Contains(u, "DRAINED") ||
		strings.Contains(u, "FAILED") || strings.Contains(u, "CRITICAL") ||
		strings.Contains(u, "ORA-"):
		return "[red]" + status + "[-]"
	default:
		return status
	}
}

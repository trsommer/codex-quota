package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) renderCompactView() string {
	if len(m.Accounts) == 0 {
		return "No accounts.\n"
	}

	accountWidth := m.compactAccountWidth()
	normalRows := make([]int, 0, len(m.Accounts))
	exhaustedRows := make([]int, 0, len(m.Accounts))

	for i, acc := range m.Accounts {
		if acc == nil {
			continue
		}
		if m.isCompactAccountExhausted(acc.Key) {
			exhaustedRows = append(exhaustedRows, i)
			continue
		}
		normalRows = append(normalRows, i)
	}

	// The compact list is rendered as: [normal rows], then a blank line
	// followed by the "Exhausted accounts" header (only when both lists
	// are non-empty), then the exhausted rows. Each row is one line.
	hasExhausted := len(exhaustedRows) > 0
	hasNormal := len(normalRows) > 0
	headerLines := 0
	if hasExhausted {
		headerLines++ // the "Exhausted accounts" line itself
		if hasNormal {
			headerLines++ // the blank separator
		}
	}

	totalRows := len(normalRows) + len(exhaustedRows)
	availableRows := m.compactAvailableRows() - headerLines
	if availableRows < 1 {
		availableRows = 1
	}

	scroll := m.CompactScroll
	if scroll < 0 {
		scroll = 0
	}
	maxScroll := totalRows - availableRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}

	var s strings.Builder
	canScroll := totalRows > availableRows
	if canScroll && scroll > 0 {
		fmt.Fprintf(&s, "%s\n",
			CompactScrollIndicatorStyle.Render(
				fmt.Sprintf("↑ %d more", scroll),
			),
		)
	}

	end := scroll + availableRows
	if end > totalRows {
		end = totalRows
	}

	// Map each row position to an account index. The exhausted section
	// starts at len(normalRows) in the position space.
	rowIndexAt := func(pos int) int {
		if pos < len(normalRows) {
			return normalRows[pos]
		}
		return exhaustedRows[pos-len(normalRows)]
	}
	isExhaustedPos := func(pos int) bool {
		return pos >= len(normalRows)
	}

	headerEmitted := false
	if canScroll && hasExhausted {
		// If the visible window starts inside (or just past) the normal
		// section, emit the header when we first reach the boundary. If
		// the window starts inside the exhausted section, emit the
		// header at the top of the visible area so the section label
		// is still shown.
		if scroll >= len(normalRows) {
			fmt.Fprintf(&s, "%s\n",
				CompactExhaustedHeaderStyle.Render("Exhausted accounts"),
			)
			headerEmitted = true
		}
	}

	for pos := scroll; pos < end; pos++ {
		if !headerEmitted && hasExhausted && isExhaustedPos(pos) {
			if hasNormal {
				s.WriteString("\n")
			}
			fmt.Fprintf(&s, "%s\n",
				CompactExhaustedHeaderStyle.Render("Exhausted accounts"),
			)
			headerEmitted = true
		}
		m.renderCompactRowLine(&s, rowIndexAt(pos), accountWidth)
	}

	if canScroll && end < totalRows {
		downCount := totalRows - end
		s.WriteString("\n")
		fmt.Fprintf(&s, "%s\n",
			CompactScrollIndicatorStyle.Render(
				fmt.Sprintf("↓ %d more", downCount),
			),
		)
	}

	return s.String()
}

// renderCompactRowLine renders a single account row, applying the same
// truncation rules used by the unsliced list renderer.
func (m Model) renderCompactRowLine(s *strings.Builder, accIdx int, accountWidth int) {
	if accIdx < 0 || accIdx >= len(m.Accounts) {
		return
	}
	acc := m.Accounts[accIdx]
	if acc == nil {
		return
	}
	row := m.renderCompactAccountRow(accIdx, acc, accountWidth)
	// Guard against style-induced line wraps on very narrow terminals.
	row = strings.ReplaceAll(row, "\n", " ")
	limit := m.preferredContentWidth()
	if limit <= 0 && m.Width > 0 {
		limit = m.Width
	}
	if limit > 0 && ansi.StringWidth(row) > limit {
		row = ansi.Cut(row, 0, limit)
	}
	s.WriteString(row)
	s.WriteString("\n")
}

func (m Model) renderCompactAccountRow(index int, acc *config.Account, accountWidth int) string {
	var s strings.Builder
	isActive := index == m.ActiveAccountIx
	prefix := "  "
	if isActive {
		prefix = "> "
	}

	name := acc.Label
	if name == "" {
		name = acc.SourceLabel()
	}
	subscribed := m.hasSubscription(acc)
	badgeWidth := m.activeSourceBadgesDisplayWidth(acc)
	nameWidth := accountWidth
	if badgeWidth > 0 {
		nameWidth = accountWidth - badgeWidth - 1
		if nameWidth < 4 {
			nameWidth = 4
		}
	}
	name = truncateLabel(name, nameWidth-1)
	alignedName := fmt.Sprintf("%-*s", nameWidth, name)
	leftWidth := ansi.StringWidth(prefix) + nameWidth + 1
	if badgeWidth > 0 {
		leftWidth += badgeWidth + 1
	}
	barWidth, percentWidth, resetWidth := m.compactRowLayout(leftWidth)

	s.WriteString(prefix)
	if badgeWidth > 0 {
		s.WriteString(m.renderActiveSourceBadges(acc, isActive))
		s.WriteString(" ")
	}
	if subscribed && isActive {
		s.WriteString(SubscribedLabelActiveStyle.Render(alignedName))
	} else if subscribed {
		s.WriteString(SubscribedLabelMutedStyle.Render(alignedName))
	} else if isActive {
		s.WriteString(TabActiveStyle.Render(alignedName))
	} else {
		s.WriteString(LabelStyle.Render(alignedName))
	}
	s.WriteString(" ")

	if err := m.ErrorsMap[acc.Key]; err != nil {
		status := truncateLabel("Error: "+err.Error(), 24)
		s.WriteString(m.renderCompactStatusRow(status, subscribed, barWidth, percentWidth, resetWidth))
		return s.String()
	}
	if m.LoadingMap[acc.Key] {
		s.WriteString(m.renderCompactStatusRow("Loading...", subscribed, barWidth, percentWidth, resetWidth))
		return s.String()
	}

	data, ok := m.UsageData[acc.Key]
	if !ok {
		s.WriteString(m.renderCompactStatusRow("Queued...", subscribed, barWidth, percentWidth, resetWidth))
		return s.String()
	}

	window, ok := compactPrimaryWindow(data)
	if !ok {
		s.WriteString(m.renderCompactStatusRow("No quota data", subscribed, barWidth, percentWidth, resetWidth))
		return s.String()
	}

	ratio := m.compactBarRatio(acc.Key, clampRatio(window.LeftPercent/100))
	s.WriteString(renderSmoothBar(barWidth, ratio, defaultBarGradientStart, defaultBarGradientEnd))
	s.WriteString(" ")
	s.WriteString(m.renderCompactPercent(fmt.Sprintf("%.0f%%", window.LeftPercent), subscribed, percentWidth))
	reset := truncateLabelStrict(formatResetText(window.ResetAt), resetWidth)
	if resetWidth > 0 && strings.TrimSpace(reset) != "" {
		s.WriteString(ResetTimeStyle.Copy().Width(resetWidth).Render(reset))
	}
	return s.String()
}

func (m Model) isCompactAccountExhausted(accountKey string) bool {
	if accountKey == "" {
		return false
	}
	if m.ExhaustedSticky[accountKey] {
		return true
	}
	if m.LoadingMap[accountKey] {
		return false
	}
	if err := m.ErrorsMap[accountKey]; err != nil {
		return false
	}

	data, ok := m.UsageData[accountKey]
	if !ok {
		return false
	}
	return isConfirmedExhausted(data)
}

func (m Model) renderCompactStatusRow(status string, subscribed bool, barWidth, percentWidth, resetWidth int) string {
	row := renderSmoothBar(barWidth, 0, defaultBarGradientStart, defaultBarGradientEnd)
	row += " "
	row += m.renderCompactPercent("...", subscribed, percentWidth)
	if resetWidth > 0 {
		row += ResetTimeStyle.Copy().Width(resetWidth).Render(truncateLabelStrict(status, resetWidth))
	}
	return TabInactiveStyle.Render(row)
}

func (m Model) renderCompactPercent(value string, subscribed bool, width int) string {
	value = truncateLabelStrict(value, width)
	style := PercentStyle.Copy().Width(width)
	if !subscribed {
		return style.Render(value)
	}

	return style.Copy().Foreground(lipgloss.Color("177")).Render(value)
}

func (m Model) compactAccountWidth() int {
	width := m.Width
	if width <= 0 {
		width = m.preferredContentWidth()
	}
	switch {
	case width >= 140:
		return 30
	case width >= 120:
		return 24
	case width >= 100:
		return 20
	case width >= 84:
		return 16
	case width >= 72:
		return 18
	default:
		return 12
	}
}

func (m Model) compactRowLayout(leftWidth int) (barWidth, percentWidth, resetWidth int) {
	barWidth = m.defaultBarWidth()
	percentWidth = 5
	resetWidth = 26

	available := m.preferredContentWidth() - leftWidth
	if available <= 0 {
		return 6, 3, 0
	}

	const (
		minBarWidth     = 6
		minPercentWidth = 4
		minResetWidth   = 0
		gapWidth        = 1
		resetMarginLeft = 2
	)

	used := barWidth + gapWidth + percentWidth + resetMarginLeft + resetWidth
	shortage := used - available
	if shortage <= 0 {
		return
	}

	reduce := func(current, minimum int) int {
		if shortage <= 0 {
			return current
		}
		can := current - minimum
		if can <= 0 {
			return current
		}
		if can > shortage {
			can = shortage
		}
		shortage -= can
		return current - can
	}

	barWidth = reduce(barWidth, minBarWidth)
	resetWidth = reduce(resetWidth, minResetWidth)
	percentWidth = reduce(percentWidth, minPercentWidth)

	return
}

// compactAvailableRows returns how many text rows the compact list can
// safely render given the current terminal height. The estimate includes
// space for the surrounding chrome (header, footer, frame padding and
// the title/footer margins). When the terminal height is unknown or
// unusually small we fall back to a large value so nothing is truncated.
func (m Model) compactAvailableRows() int {
	const chromeLines = 9
	if m.Height <= 0 {
		return 1 << 30
	}
	available := m.Height - chromeLines
	if available < 3 {
		return 3
	}
	return available
}

// clampCompactScroll keeps CompactScroll within the valid range for the
// current list size and terminal height.
func (m *Model) clampCompactScroll() {
	if !m.CompactMode || len(m.Accounts) == 0 {
		m.CompactScroll = 0
		return
	}
	total := len(m.Accounts)
	available := m.compactAvailableRows()
	// Subtract space taken by the exhausted header when both sections
	// could be present; this matches the renderer.
	hasExhausted := false
	for _, acc := range m.Accounts {
		if acc != nil && m.isCompactAccountExhausted(acc.Key) {
			hasExhausted = true
			break
		}
	}
	headerLines := 0
	if hasExhausted {
		headerLines = 2
		hasNormal := false
		for _, acc := range m.Accounts {
			if acc != nil && !m.isCompactAccountExhausted(acc.Key) {
				hasNormal = true
				break
			}
		}
		if !hasNormal {
			headerLines = 1
		}
	}
	visible := available - headerLines
	if visible < 1 {
		visible = 1
	}
	maxScroll := total - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.CompactScroll < 0 {
		m.CompactScroll = 0
	}
	if m.CompactScroll > maxScroll {
		m.CompactScroll = maxScroll
	}
}

// ensureCompactActiveVisible adjusts CompactScroll so the currently
// active account is inside the visible window. Call after changing
// ActiveAccountIx in compact mode.
func (m *Model) ensureCompactActiveVisible() {
	if !m.CompactMode || len(m.Accounts) == 0 {
		m.CompactScroll = 0
		return
	}
	pos := m.compactActiveRowPosition()
	if pos < 0 {
		m.CompactScroll = 0
		return
	}
	visible := m.compactAvailableRows()
	hasExhausted := false
	for _, acc := range m.Accounts {
		if acc != nil && m.isCompactAccountExhausted(acc.Key) {
			hasExhausted = true
			break
		}
	}
	headerLines := 0
	if hasExhausted {
		headerLines = 2
		hasNormal := false
		for _, acc := range m.Accounts {
			if acc != nil && !m.isCompactAccountExhausted(acc.Key) {
				hasNormal = true
				break
			}
		}
		if !hasNormal {
			headerLines = 1
		}
	}
	visible -= headerLines
	if visible < 1 {
		visible = 1
	}
	total := len(m.Accounts)
	maxScroll := total - visible
	if maxScroll < 0 {
		maxScroll = 0
	}

	if m.CompactScroll > pos {
		m.CompactScroll = pos
	}
	if m.CompactScroll < pos-visible+1 {
		m.CompactScroll = pos - visible + 1
	}
	if m.CompactScroll > maxScroll {
		m.CompactScroll = maxScroll
	}
	if m.CompactScroll < 0 {
		m.CompactScroll = 0
	}
}

// compactActiveRowPosition returns the index of the active account in
// the same order the compact list renders rows (normal accounts first,
// then exhausted accounts). It returns -1 when the active account
// cannot be located.
func (m Model) compactActiveRowPosition() int {
	if m.ActiveAccountIx < 0 || m.ActiveAccountIx >= len(m.Accounts) {
		return -1
	}
	active := m.Accounts[m.ActiveAccountIx]
	if active == nil {
		return -1
	}
	pos := 0
	for _, acc := range m.Accounts {
		if acc == nil {
			continue
		}
		if m.isCompactAccountExhausted(acc.Key) {
			continue
		}
		if acc.Key == active.Key {
			return pos
		}
		pos++
	}
	pos = 0
	for _, acc := range m.Accounts {
		if acc == nil {
			continue
		}
		if !m.isCompactAccountExhausted(acc.Key) {
			continue
		}
		if acc.Key == active.Key {
			// Skip past the normal section count.
			normalCount := 0
			for _, a := range m.Accounts {
				if a != nil && !m.isCompactAccountExhausted(a.Key) {
					normalCount++
				}
			}
			return normalCount + pos
		}
		pos++
	}
	return -1
}

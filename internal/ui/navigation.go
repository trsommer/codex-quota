package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) activeAccount() *config.Account {
	if len(m.Accounts) == 0 {
		return nil
	}
	if m.ActiveAccountIx < 0 || m.ActiveAccountIx >= len(m.Accounts) {
		return nil
	}
	return m.Accounts[m.ActiveAccountIx]
}

func (m Model) activeAccountKey() string {
	account := m.activeAccount()
	if account == nil {
		return ""
	}
	return account.Key
}

func (m Model) compactVisualOrderIndices() []int {
	if len(m.Accounts) == 0 {
		return nil
	}

	normal := make([]int, 0, len(m.Accounts))
	exhausted := make([]int, 0, len(m.Accounts))
	for i, acc := range m.Accounts {
		if acc == nil {
			continue
		}
		if m.isCompactAccountExhausted(acc.Key) {
			exhausted = append(exhausted, i)
		} else {
			normal = append(normal, i)
		}
	}
	return append(normal, exhausted...)
}

func (m *Model) moveActiveAccountCompact(delta int) {
	order := m.compactVisualOrderIndices()
	if len(order) == 0 {
		return
	}

	pos := -1
	for i, idx := range order {
		if idx == m.ActiveAccountIx {
			pos = i
			break
		}
	}
	if pos == -1 {
		m.ActiveAccountIx = order[0]
		m.ensureCompactActiveVisible()
		return
	}

	next := (pos + delta) % len(order)
	if next < 0 {
		next += len(order)
	}
	m.ActiveAccountIx = order[next]
	m.ensureCompactActiveVisible()
}

func (m *Model) syncActiveAccount() {
	m.Loading = true
	m.Err = nil
	m.resetDeleteState()
	m.resetApplyState()
	m.Notice = ""
	m.clearTabWindowAnimations()

	if acc := m.activeAccount(); acc != nil {
		if data, ok := m.UsageData[acc.Key]; ok {
			m.Data = data
			m.Loading = false
			m.Err = m.ErrorsMap[acc.Key]
			if !m.CompactMode {
				m.startTabWindowAnimationsFromZero(acc.Key, data, tabSwitchAnimationDuration)
			}
			m.ensureCompactActiveVisible()
			return
		}
	}
	m.Data = api.UsageData{}
}

func (m *Model) normalizeActiveAccountForView(activeKey string) {
	activeKey = strings.TrimSpace(activeKey)
	if len(m.Accounts) == 0 {
		m.ActiveAccountIx = 0
		return
	}

	if activeKey != "" {
		for i, account := range m.Accounts {
			if account != nil && account.Key == activeKey {
				m.ActiveAccountIx = i
				m.ensureCompactActiveVisible()
				return
			}
		}
	}

	if m.CompactMode {
		if order := m.compactVisualOrderIndices(); len(order) > 0 {
			m.ActiveAccountIx = order[0]
			m.ensureCompactActiveVisible()
			return
		}
	}

	m.ActiveAccountIx = 0
}

func (m *Model) syncAndFetchActiveAccount() tea.Cmd {
	m.syncActiveAccount()
	return tea.Batch(m.fetchNextCmd(), m.ensureAnimationTickCmd())
}

package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

// makeAccounts returns a deterministic slice of accounts used by the
// scroll tests. The label is intentionally short so each row collapses
// to a single line, mirroring the rendering of the production list.
func makeAccounts(n int) []*config.Account {
	accs := make([]*config.Account, 0, n)
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("acc-%02d", i)
		accs = append(accs, &config.Account{
			Key:       key,
			Label:     fmt.Sprintf("user%02d@example.com", i),
			Email:     fmt.Sprintf("user%02d@example.com", i),
			AccountID: fmt.Sprintf("id-%02d", i),
			Source:    config.SourceManaged,
			Writable:  true,
		})
	}
	return accs
}

// markExhaustedSticky forces an account to show up in the exhausted
// section without going through the network.
func markExhaustedSticky(m *Model, key string) {
	if m.ExhaustedSticky == nil {
		m.ExhaustedSticky = map[string]bool{}
	}
	m.ExhaustedSticky[key] = true
}

func newCompactModelForScroll(t *testing.T, accounts []*config.Account, height int) Model {
	t.Helper()
	m := InitialModel(accounts, map[string][]string{}, map[string][]string{}, true)
	m.Width = 100
	m.Height = height
	m.Loading = false
	m.ActiveAccountIx = 0
	for _, acc := range accounts {
		if acc == nil {
			continue
		}
		m.UsageData[acc.Key] = api.UsageData{
			Windows: []api.QuotaWindow{{
				Label:       "Weekly usage limit",
				WindowSec:   604800,
				LeftPercent: 50.0,
				ResetAt:     time.Now().Add(24 * time.Hour),
			}},
		}
	}
	return m
}

func TestRenderCompactView_OverflowsShowScrollIndicator(t *testing.T) {
	accounts := makeAccounts(40)
	m := newCompactModelForScroll(t, accounts, 12) // small terminal

	out := m.renderCompactView()
	stripped := ansi.Strip(out)

	if !strings.Contains(stripped, "more") {
		t.Fatalf("expected scroll indicator with 'more' text in output:\n%s", stripped)
	}
	// Should not contain every account label.
	if strings.Count(stripped, "@example.com") >= len(accounts) {
		t.Fatalf("expected output to omit some accounts when overflowing, got %d labels",
			strings.Count(stripped, "@example.com"))
	}
}

func TestRenderCompactView_FitsNoScrollIndicators(t *testing.T) {
	accounts := makeAccounts(4)
	m := newCompactModelForScroll(t, accounts, 60) // tall terminal

	out := ansi.Strip(m.renderCompactView())
	if strings.Contains(out, "more") {
		t.Fatalf("expected no scroll indicators when list fits, got:\n%s", out)
	}
	for i := 0; i < len(accounts); i++ {
		want := fmt.Sprintf("user%02d@example.com", i)
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestRenderCompactView_ScrollIndicatorCountMatchesHiddenRows(t *testing.T) {
	accounts := makeAccounts(30)
	m := newCompactModelForScroll(t, accounts, 12)

	m.CompactScroll = 10
	out := ansi.Strip(m.renderCompactView())

	if !strings.Contains(out, "↑ 10 more") {
		t.Fatalf("expected '↑ 10 more' indicator, got:\n%s", out)
	}
}

func TestRenderCompactView_ScrollingDownHidesTopRows(t *testing.T) {
	accounts := makeAccounts(40)
	m := newCompactModelForScroll(t, accounts, 12)
	m.CompactScroll = 5

	out := ansi.Strip(m.renderCompactView())
	if strings.Contains(out, "user00@example.com") {
		t.Fatalf("expected user00 to be scrolled out of view, got:\n%s", out)
	}
}

func TestRenderCompactView_ExhaustedHeaderVisibleWhenScrolled(t *testing.T) {
	accounts := makeAccounts(40)
	m := newCompactModelForScroll(t, accounts, 12)
	markExhaustedSticky(&m, accounts[0].Key)

	// Scroll to the maximum so the bottom of the visible window lands
	// on the exhausted account. The header should still appear inside
	// the rendered output.
	m.clampCompactScroll()
	m.CompactScroll = 1000
	m.clampCompactScroll()
	out := ansi.Strip(m.renderCompactView())
	if !strings.Contains(out, "Exhausted accounts") {
		t.Fatalf("expected exhausted section header in output, got:\n%s", out)
	}
}

func TestEnsureCompactActiveVisible_KeepsActiveInView(t *testing.T) {
	accounts := makeAccounts(40)
	m := newCompactModelForScroll(t, accounts, 12)
	m.ActiveAccountIx = 25
	m.ensureCompactActiveVisible()

	out := ansi.Strip(m.renderCompactView())
	activeLabel := accounts[m.ActiveAccountIx].Label
	found := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, activeLabel) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected active label %q in visible window, got:\n%s", activeLabel, out)
	}
}

func TestClampCompactScroll_BoundsToListSize(t *testing.T) {
	accounts := makeAccounts(40)
	m := newCompactModelForScroll(t, accounts, 12)

	m.CompactScroll = 1000
	m.clampCompactScroll()
	if m.CompactScroll < 0 {
		t.Fatalf("expected non-negative scroll, got %d", m.CompactScroll)
	}
	if m.CompactScroll > len(accounts) {
		t.Fatalf("expected scroll <= total rows, got %d", m.CompactScroll)
	}
}

func TestMoveActiveAccountCompact_AdjustsScroll(t *testing.T) {
	accounts := makeAccounts(40)
	m := newCompactModelForScroll(t, accounts, 12)

	for i := 0; i < 25; i++ {
		m.moveActiveAccountCompact(1)
	}

	out := ansi.Strip(m.renderCompactView())
	activeLabel := accounts[m.ActiveAccountIx].Label
	found := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, activeLabel) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected active account %q to be visible after navigating down, got:\n%s",
			activeLabel, out)
	}
}

func TestMoveActiveAccountCompact_ScrollsUp(t *testing.T) {
	accounts := makeAccounts(40)
	m := newCompactModelForScroll(t, accounts, 12)

	for i := 0; i < 35; i++ {
		m.moveActiveAccountCompact(1)
	}
	m.moveActiveAccountCompact(-1)
	m.moveActiveAccountCompact(-1)

	out := ansi.Strip(m.renderCompactView())
	activeLabel := accounts[m.ActiveAccountIx].Label
	found := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, activeLabel) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected active account %q to be visible after navigating up, got:\n%s",
			activeLabel, out)
	}
}

func TestAccountsMsg_ResetsScroll(t *testing.T) {
	accounts := makeAccounts(40)
	m := newCompactModelForScroll(t, accounts, 12)
	m.CompactScroll = 15

	updated, _ := m.Update(AccountsMsg{
		Accounts:                accounts,
		SourcesByAccountID:      map[string][]string{},
		ActiveSourcesByIdentity: map[string][]string{},
	})
	got := updated.(Model)
	if got.CompactScroll != 0 {
		t.Fatalf("expected CompactScroll to reset to 0 on AccountsMsg, got %d", got.CompactScroll)
	}
}

func TestViewModeToggle_ResetsScroll(t *testing.T) {
	accounts := makeAccounts(40)
	m := newCompactModelForScroll(t, accounts, 12)
	m.CompactScroll = 20

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	got := updated.(Model)
	if got.CompactScroll != 0 {
		t.Fatalf("expected CompactScroll to reset on view toggle, got %d", got.CompactScroll)
	}
}

func TestRenderCompactView_OnlyExhaustedStillShowsHeader(t *testing.T) {
	accounts := makeAccounts(20)
	m := newCompactModelForScroll(t, accounts, 60)
	// Mark every account as exhausted so the only section is the
	// exhausted one (no blank separator before its header).
	for _, acc := range accounts {
		markExhaustedSticky(&m, acc.Key)
	}
	out := ansi.Strip(m.renderCompactView())
	if !strings.Contains(out, "Exhausted accounts") {
		t.Fatalf("expected exhausted header even when only exhausted accounts exist, got:\n%s", out)
	}
}

package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"sqvue/internal/db"
)

func (m Model) openProfilePicker() (Model, tea.Cmd) {
	if len(m.profiles) == 0 {
		m.status = "no configured connection profiles"
		return m, nil
	}
	m.profileCursor = 0
	for i, profile := range m.profiles {
		if profile.Name == m.profileName {
			m.profileCursor = i
			break
		}
	}
	m.activeOverlay, m.status = overlayProfilePicker, "select a connection profile"
	return m, nil
}

func (m Model) handleProfilePickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.activeOverlay = overlayNone
		m.restoreBrowseStatus()
		return m, nil
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Down):
		m.profileCursor = min(m.profileCursor+1, len(m.profiles)-1)
	case key.Matches(msg, m.keys.Up):
		m.profileCursor = max(0, m.profileCursor-1)
	case key.Matches(msg, m.keys.Confirm):
		if len(m.profiles) == 0 {
			return m, nil
		}
		profile := m.profiles[m.profileCursor]
		if profile.Name == m.profileName {
			m.activeOverlay = overlayNone
			m.restoreBrowseStatus()
			return m, nil
		}
		m.closeTableStream()
		m.closeQueryStream()
		m.nextRequestID() // Make all outstanding loads stale before reconnecting.
		m.reconnectID++
		m.activeOverlay, m.loading = overlayNone, true
		m.status = "connecting to profile " + sanitizeText(profile.Name) + "..."
		return m, connectProfileCmd(profile, m.reconnectID)
	}
	return m, nil
}

func connectProfileCmd(profile ConnectionProfile, reconnectID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), profile.Timeout)
		defer cancel()
		client, err := db.NewClientWithConfig(ctx, profile.Config)
		return profileConnectedMsg{reconnectID: reconnectID, profile: profile, client: client, err: err}
	}
}

func (m Model) handleProfileConnected(msg profileConnectedMsg) (Model, tea.Cmd) {
	if msg.reconnectID != m.reconnectID {
		if msg.client != nil {
			_ = msg.client.Close()
		}
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		m.status = "connection to " + sanitizeText(msg.profile.Name) + " failed; still using " + sanitizeText(m.profileName) + ": " + msg.err.Error()
		m.lastErr = msg.err
		return m, nil
	}
	previous := m.client
	m.client, m.profileName, m.timeout = msg.client, msg.profile.Name, msg.profile.Timeout
	if previous != nil {
		_ = previous.Close()
	}
	m.resetAfterProfileSwitch()
	m.loading, m.status = true, "loading schemas for profile "+sanitizeText(msg.profile.Name)+"..."
	return m, loadSchemasCmd(m.client, m.timeout, m.nextRequestID())
}

func (m *Model) resetAfterProfileSwitch() {
	m.schemas, m.schema, m.schemaCursor, m.schemaScroll = nil, 0, 0, 0
	m.allTables, m.tables, m.selected, m.scroll = nil, nil, 0, 0
	m.mode, m.columns, m.rows, m.tableInfo = modeValues, nil, nil, db.TableInfo{}
	m.page, m.hasNextPage, m.rowCursor, m.cellCursor, m.pendingRowMoves = 0, false, 0, 0, 0
	m.rowCounts, m.browseRowCounts = make(map[string]int64), make(map[string]int64)
	m.browseFilters, m.browseSort, m.visibleColumns, m.visibleColumnKey = nil, db.SortSpec{}, nil, ""
	m.queryActive, m.queryRows, m.queryBaseRows, m.querySQL, m.querySourceSQL, m.queryStreaming, m.querySort = false, nil, nil, "", "", false, db.SortSpec{}
	m.filterInput.SetValue("")
	m.sqlInput.SetValue("")
	m.focused, m.lastErr = false, nil
}

func renderProfilePickerModal(m Model) string {
	lines := make([]string, 0, len(m.profiles)+2)
	for i, profile := range m.profiles {
		prefix := "  "
		if i == m.profileCursor {
			prefix = "> "
		}
		name := sanitizeText(profile.Name)
		if profile.Name == m.profileName {
			name += " (current)"
		}
		lines = append(lines, prefix+name)
	}
	if len(lines) == 0 {
		lines = append(lines, "(no configured profiles)")
	}
	lines = append(lines, "", m.theme.Muted.Render("Enter reconnects · Esc cancels"))
	width := detailDialogWidth(m)
	panelLines := strings.Split(m.theme.Dialog.Width(width).Render(strings.Join(lines, "\n")), "\n")
	return titledDialog(m, panelLines, ansi.StringWidth(panelLines[0]), "Connection profiles")
}

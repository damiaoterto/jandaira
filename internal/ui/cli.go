package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/damiaoterto/jandaira/internal/swarm"
)

// ── Theme & Layout ────────────────────────────────────────────────────────
type Theme struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color
	Success   lipgloss.Color
	Info      lipgloss.Color
	Warning   lipgloss.Color
	Error     lipgloss.Color
	Text      lipgloss.Color
	Muted     lipgloss.Color
	Surface   lipgloss.Color
	BgDark    lipgloss.Color
}

var DefaultTheme = Theme{
	Primary:   lipgloss.Color("#FFD166"), // Amber
	Secondary: lipgloss.Color("#F5A623"), // Gold
	Accent:    lipgloss.Color("#F78FA7"), // Pink
	Success:   lipgloss.Color("#06D6A0"), // Green
	Info:      lipgloss.Color("#48CAE4"), // Cyan
	Warning:   lipgloss.Color("#FF8C00"), // Orange
	Error:     lipgloss.Color("#EF476F"), // Red
	Text:      lipgloss.Color("#F8F8F2"), // Light Text
	Muted:     lipgloss.Color("#6C6C8A"), // Gray
	Surface:   lipgloss.Color("#2D2B55"), // Purple-ish Dark
	BgDark:    lipgloss.Color("#1A1A2E"), // Deep Background
}

type Styles struct {
	// Layout
	App lipgloss.Style

	// Typography
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Status   lipgloss.Style

	// Bubbles
	UserBubble    lipgloss.Style
	QueenBubble   lipgloss.Style
	ErrorBubble   lipgloss.Style
	SystemMessage lipgloss.Style

	// Approvals
	ApprovalBox    lipgloss.Style
	WarningLabel   lipgloss.Style
	ApprovalKey    lipgloss.Style
	ApprovalValue  lipgloss.Style
	ApprovalPrompt lipgloss.Style

	// Input
	InputBox   lipgloss.Style
	InputLabel lipgloss.Style
	Footer     lipgloss.Style
}

func DefaultStyles() *Styles {
	t := DefaultTheme

	return &Styles{
		App: lipgloss.NewStyle().Padding(1, 2),

		Title: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Padding(0, 1),

		Subtitle: lipgloss.NewStyle().
			Foreground(t.Muted).
			Italic(true),

		Status: lipgloss.NewStyle().
			Foreground(t.Primary).
			Italic(true),

		UserBubble: lipgloss.NewStyle().
			Background(t.Info).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1).
			MarginBottom(1).
			MarginTop(1),

		QueenBubble: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Accent).
			Padding(0, 1).
			MarginBottom(1).
			MarginTop(1),

		ErrorBubble: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Error).
			Foreground(t.Error).
			Padding(0, 1).
			MarginBottom(1).
			MarginTop(1),

		SystemMessage: lipgloss.NewStyle().
			Foreground(t.Muted).
			Italic(true).
			MarginBottom(1).
			MarginTop(1),

		ApprovalBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Warning).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1),

		WarningLabel: lipgloss.NewStyle().
			Foreground(t.Warning).
			Bold(true),

		ApprovalKey: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		ApprovalValue: lipgloss.NewStyle().
			Foreground(t.Text),

		ApprovalPrompt: lipgloss.NewStyle().
			Foreground(t.Warning).
			Bold(true).
			MarginTop(1),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Secondary).
			Padding(0, 1).
			MarginTop(1),

		InputLabel: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		Footer: lipgloss.NewStyle().
			Foreground(t.Muted).
			Italic(true).
			MarginTop(1),
	}
}

// ── Types ──────────────────────────────────────────────────────────────────

// PermissionMsg is sent by the Queen goroutine when it requires Apicultor approval.
type PermissionMsg struct {
	ToolName string
	Args     string
}

type ResultMsg struct {
	content string
	err     error
}

type StatusMsg string

type HistoryMsg struct {
	Role    string // "user", "queen", "error", "system"
	Content string
}

type CLIModel struct {
	Queen      *swarm.Queen
	Pipeline   []swarm.Specialist
	TextInput  textinput.Model
	Spinner    spinner.Model
	History    []HistoryMsg
	IsWorking  bool
	StatusLine string

	// Apicultor (beekeeper) Human-in-the-Loop state
	WaitingApproval bool
	ApprovalTool    string
	ApprovalArgs    string

	// Layout state
	width  int
	styles *Styles
}

// ── Initialization ─────────────────────────────────────────────────────────

func InitialModel(q *swarm.Queen, p []swarm.Specialist) CLIModel {
	styles := DefaultStyles()

	ti := textinput.New()
	ti.Placeholder = "Diga à Rainha o que deseja fazer..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 52
	ti.PromptStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Secondary)
	ti.TextStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Primary)

	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(DefaultTheme.Secondary)

	return CLIModel{
		Queen:      q,
		Pipeline:   p,
		TextInput:  ti,
		Spinner:    s,
		History: []HistoryMsg{
			{Role: "system", Content: "✦ A Colmeia Jandaira despertou. As operárias aguardam as suas ordens."},
		},
		IsWorking: false,
		styles:    styles,
	}
}

func (m CLIModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.Spinner.Tick)
}

// ── Update ─────────────────────────────────────────────────────────────────

func (m CLIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		// Reserve space for padding and borders
		if m.width > 12 {
			m.TextInput.Width = m.width - 12
		}
		return m, nil

	case tea.KeyMsg:
		// ── Apicultor approval intercept ───────────────────────
		if m.WaitingApproval {
			switch msg.String() {
			case "s", "S", "y", "Y":
				m.WaitingApproval = false
				m.Queen.ApprovalChan <- true
				m.StatusLine = "✅ Permissão concedida. A Rainha retomou a tarefa..."
				return m, nil
			case "n", "N":
				m.WaitingApproval = false
				m.Queen.ApprovalChan <- false
				m.StatusLine = "🚫 Ação bloqueada. A Rainha está a reconsiderar..."
				return m, nil
			case "ctrl+c", "esc":
				return m, tea.Quit
			}
			// Ignore all other keys while waiting for approval
			return m, nil
		}

		// ── Normal input flow ──────────────────────────────────
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.IsWorking || m.TextInput.Value() == "" {
				return m, nil
			}
			goal := m.TextInput.Value()
			m.TextInput.SetValue("")

			m.History = append(m.History, HistoryMsg{Role: "user", Content: goal})
			m.IsWorking = true
			m.StatusLine = "A Rainha está a analisar a tarefa..."
			cmds = append(cmds, m.runGoal(goal))
		}

	case PermissionMsg:
		m.WaitingApproval = true
		m.ApprovalTool = msg.ToolName
		m.ApprovalArgs = msg.Args
		return m, nil

	case ResultMsg:
		m.IsWorking = false
		if msg.err != nil {
			m.History = append(m.History, HistoryMsg{Role: "error", Content: msg.err.Error()})
		} else {
			m.History = append(m.History, HistoryMsg{Role: "queen", Content: msg.content})
		}
		m.StatusLine = ""

	case StatusMsg:
		m.StatusLine = string(msg)

	case spinner.TickMsg:
		m.Spinner, cmd = m.Spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.TextInput, cmd = m.TextInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m CLIModel) runGoal(goal string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		resultChan, errChan := m.Queen.DispatchWorkflow(ctx, "enxame-alfa", goal, m.Pipeline)

		select {
		case res := <-resultChan:
			return ResultMsg{content: res}
		case err := <-errChan:
			return ResultMsg{err: err}
		case <-ctx.Done():
			return ResultMsg{err: ctx.Err()}
		}
	}
}

// ── View Rendering ─────────────────────────────────────────────────────────

func (m CLIModel) View() string {
	width := m.width
	if width == 0 {
		width = 80 // Reliable default before initial WindowSizeMsg
	}

	var b strings.Builder

	// Padding allowance
	contentWidth := width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// 1. Banner
	bannerTitle := lipgloss.JoinVertical(
		lipgloss.Center,
		m.styles.Title.Render("🍯 Jandaira Swarm OS 🍯"),
		m.styles.Subtitle.Render("Swarm Intelligence · Powered by Go"),
	)

	bannerBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(DefaultTheme.Secondary).
		Width(contentWidth).
		Align(lipgloss.Center).
		MarginBottom(1)

	b.WriteString(bannerBox.Render(bannerTitle))
	b.WriteString("\n")

	// 2. Chat History
	b.WriteString(m.renderHistory(contentWidth))

	// 3. Footer / Input / HitL Status
	if m.WaitingApproval {
		b.WriteString("\n" + m.renderApprovalBox(contentWidth))
	} else if m.IsWorking {
		spinnerLine := fmt.Sprintf("%s %s", m.Spinner.View(), m.styles.Status.Render(m.StatusLine))
		spinnerText := lipgloss.PlaceHorizontal(contentWidth, lipgloss.Left, spinnerLine)
		b.WriteString("\n" + spinnerText + "\n")
	} else {
		b.WriteString("\n" + m.renderInputBox(contentWidth))
	}

	// Wrapper guarantees container padding logic
	return m.styles.App.Render(b.String())
}

// wrapText adds dynamic word-wrapping functionality
func (m CLIModel) wrapText(text string, maxWidth int) string {
	if lipgloss.Width(text) > maxWidth {
		return lipgloss.NewStyle().Width(maxWidth).Render(text)
	}
	return text
}

func (m CLIModel) renderHistory(width int) string {
	var b strings.Builder

	maxBubbleWidth := int(float64(width) * 0.8) // Bubbles span up to 80% screen width
	if maxBubbleWidth < 30 {
		maxBubbleWidth = width
	}

	for _, msg := range m.History {
		b.WriteString("\n")
		switch msg.Role {
		case "system":
			sysContent := m.styles.SystemMessage.Render(msg.Content)
			row := lipgloss.PlaceHorizontal(width, lipgloss.Center, sysContent)
			b.WriteString(row)

		case "user":
			content := m.wrapText(msg.Content, maxBubbleWidth)
			bubble := m.styles.UserBubble.Render(content)
			row := lipgloss.PlaceHorizontal(width, lipgloss.Right, bubble)
			b.WriteString(row)

		case "queen":
			header := lipgloss.NewStyle().Foreground(DefaultTheme.Accent).Bold(true).Render("👑 Rainha:")
			content := m.wrapText(msg.Content, maxBubbleWidth)
			bubble := m.styles.QueenBubble.Render(content)
			body := lipgloss.JoinVertical(lipgloss.Left, header, bubble)
			row := lipgloss.PlaceHorizontal(width, lipgloss.Left, body)
			b.WriteString(row)

		case "error":
			header := lipgloss.NewStyle().Foreground(DefaultTheme.Error).Bold(true).Render("⚠️ Erro:")
			content := m.wrapText(msg.Content, maxBubbleWidth)
			bubble := m.styles.ErrorBubble.Render(content)
			body := lipgloss.JoinVertical(lipgloss.Left, header, bubble)
			row := lipgloss.PlaceHorizontal(width, lipgloss.Left, body)
			b.WriteString(row)
		}
	}
	return b.String()
}

func (m CLIModel) renderApprovalBox(width int) string {
	header := fmt.Sprintf("%s ⚠️  A IA requisita usar a ferramenta: %s",
		m.Spinner.View(),
		m.styles.WarningLabel.Render(m.ApprovalTool),
	)

	argsBlock := m.formatApprovalArgs(m.ApprovalArgs)
	prompt := m.styles.ApprovalPrompt.Render("👨\u200d🌾 Você autoriza? (S = Sim / N = Não)")

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		argsBlock,
		prompt,
	)

	return m.styles.ApprovalBox.Copy().Width(width).Render(content) + "\n"
}

func (m CLIModel) formatApprovalArgs(rawJSON string) string {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		return lipgloss.NewStyle().Foreground(DefaultTheme.Text).Italic(true).Render(rawJSON)
	}

	var lines []string
	codeBg := lipgloss.NewStyle().Background(DefaultTheme.BgDark).Padding(0, 1)

	for k, v := range parsed {
		strVal := fmt.Sprintf("%v", v)
		if strings.Contains(strVal, "\n") {
			indented := strings.ReplaceAll(strVal, "\n", "\n  ")
			lines = append(lines,
				m.styles.ApprovalKey.Render("▸ "+k+":"),
				codeBg.Render("  "+indented),
			)
		} else {
			lines = append(lines,
				m.styles.ApprovalKey.Render("▸ "+k+": ")+m.styles.ApprovalValue.Render(strVal),
			)
		}
	}
	return strings.Join(lines, "\n")
}

func (m CLIModel) renderInputBox(width int) string {
	inputContent := m.styles.InputLabel.Render("🐝 Objetivo ") + m.TextInput.View()
	box := m.styles.InputBox.Copy().Width(width).Render(inputContent)
	tips := m.styles.Footer.Copy().Width(width).Align(lipgloss.Center).Render("↵ enviar • esc sair")
	return lipgloss.JoinVertical(lipgloss.Left, box, tips) + "\n"
}

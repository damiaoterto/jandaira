package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/damiaoterto/jandaira/internal/i18n"
	"github.com/damiaoterto/jandaira/internal/swarm"
)

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
	HeaderBox lipgloss.Style
}

func DefaultStyles() *Styles {
	t := DefaultTheme

	return &Styles{
		App: lipgloss.NewStyle().Padding(1, 2),

		HeaderBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(t.Secondary).
			Align(lipgloss.Center),

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
			Background(lipgloss.Color("#2A3A4A")).
			Foreground(t.Text).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Info).
			Padding(0, 1).
			MarginBottom(1).
			MarginTop(1),

		QueenBubble: lipgloss.NewStyle().
			Background(lipgloss.Color("#2D1D3A")).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(t.Accent).
			Padding(0, 1).
			MarginBottom(1).
			MarginTop(1),

		ErrorBubble: lipgloss.NewStyle().
			Background(lipgloss.Color("#3A1A1A")).
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
			Border(lipgloss.DoubleBorder()).
			BorderForeground(t.Warning).
			Background(lipgloss.Color("#2A1A05")).
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
			Background(t.BgDark).
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

// PermissionMsg is sent by the Queen goroutine when it requires Apicultor approval.
type PermissionMsg struct {
	ToolName string
	Args     string
}

type ResultMsg struct {
	content string
	err     error
}

type PlanReadyMsg struct {
	pipeline []swarm.Specialist
	goal     string
}

type StatusMsg string

type HistoryMsg struct {
	Role    string // "user", "queen", "error", "system"
	Content string
}

type CLIModel struct {
	Queen           *swarm.Queen
	SwarmName       string
	TextInput       textinput.Model
	Spinner         spinner.Model
	viewport        viewport.Model
	History         []HistoryMsg
	IsWorking       bool
	StatusLine      string
	WaitingApproval bool
	ApprovalTool    string
	ApprovalArgs    string
	PlanSummary     string
	width           int
	height          int
	ready           bool
	styles          *Styles
	maxWorkers      int
}

func InitialModel(q *swarm.Queen, swarmName string, maxWorkers int) CLIModel {
	styles := DefaultStyles()

	ti := textinput.New()
	ti.Placeholder = i18n.T("cli_prompt_placeholder")
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 52
	ti.PromptStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Secondary)
	ti.TextStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Primary)

	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(DefaultTheme.Secondary)

	return CLIModel{
		Queen:     q,
		SwarmName: swarmName,
		TextInput: ti,
		Spinner:   s,
		History: []HistoryMsg{
			{Role: "system", Content: i18n.T("cli_greeting")},
		},
		IsWorking:  false,
		styles:     styles,
		maxWorkers: maxWorkers,
	}
}

func (m CLIModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.Spinner.Tick)
}

func (m CLIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = updateScreenLayout(m)
		if m.width > 12 {
			m.TextInput.Width = m.width - 12
		}
		return m, nil

	case tea.KeyMsg:
		if m.WaitingApproval {
			switch msg.String() {
			case "s", "S", "y", "Y":
				m.WaitingApproval = false
				m.PlanSummary = ""
				m.Queen.ApprovalChan <- true
				m.StatusLine = "✅ Permissão concedida. A Rainha retomou a tarefa..."
				return m, nil
			case "n", "N":
				m.WaitingApproval = false
				m.PlanSummary = ""
				m.Queen.ApprovalChan <- false
				m.StatusLine = "🚫 Ação bloqueada. A Rainha está a reconsiderar..."
				return m, nil
			case "ctrl+c", "esc":
				return m, tea.Quit
			}
			return m, nil
		}

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
			m.StatusLine = i18n.T("cli_msg_processing")
			cmds = append(cmds, m.runGoal(goal))
		}

	case PermissionMsg:
		m.WaitingApproval = true
		m.ApprovalTool = msg.ToolName
		m.ApprovalArgs = msg.Args
		return m, nil

	case PlanReadyMsg:
		var summary strings.Builder
		summary.WriteString(fmt.Sprintf("🐝 Enxame planejado com %d operária(s):\n", len(msg.pipeline)))
		for _, spec := range msg.pipeline {
			roleSnippet := spec.SystemPrompt
			if len(roleSnippet) > 100 {
				roleSnippet = roleSnippet[:97] + "..."
			}
			summary.WriteString(fmt.Sprintf("\n▸ %s\n  Papel: %s\n  Ferramentas: %s\n", spec.Name, roleSnippet, strings.Join(spec.AllowedTools, ", ")))
		}
		
		m.PlanSummary = summary.String()
		
		cmds = append(cmds, m.executePipeline(msg.goal, msg.pipeline))

	case ResultMsg:
		m.IsWorking = false
		m.PlanSummary = ""
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

	// Update viewport state
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	// Layout and chat sync always forced
	m = updateScreenLayout(m)

	return m, tea.Batch(cmds...)
}

func (m CLIModel) runGoal(goal string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		dynamicPipeline, err := m.Queen.AssembleSwarm(ctx, goal, m.maxWorkers)
		if err != nil {
			return ResultMsg{err: fmt.Errorf("error planning the swarm: %v", err)}
		}

		return PlanReadyMsg{pipeline: dynamicPipeline, goal: goal}
	}
}

func (m CLIModel) executePipeline(goal string, dynamicPipeline []swarm.Specialist) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		resultChan, errChan := m.Queen.DispatchWorkflow(ctx, m.SwarmName, goal, dynamicPipeline)

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

func (m CLIModel) renderHeader(width int) string {
	bannerTitle := lipgloss.JoinVertical(
		lipgloss.Center,
		m.styles.Title.Render(i18n.T("cli_header_title")),
		m.styles.Subtitle.Render(i18n.T("cli_header_subtitle")),
	)
	return m.styles.HeaderBox.Width(width).Render(bannerTitle)
}

func (m CLIModel) renderFooter(width int) string {
	var b strings.Builder

	if m.PlanSummary != "" {
		summaryBox := m.styles.SystemMessage.Width(width).Render(m.PlanSummary)
		b.WriteString("\n" + summaryBox)
	}

	if m.WaitingApproval {
		b.WriteString("\n" + m.renderApprovalBox(width))
	} else if m.IsWorking {
		spinnerLine := fmt.Sprintf("%s %s", m.Spinner.View(), m.styles.Status.Render(m.StatusLine))
		spinnerText := lipgloss.PlaceHorizontal(width, lipgloss.Left, spinnerLine)
		b.WriteString("\n\n" + spinnerText)
	} else {
		b.WriteString("\n\n" + m.renderInputBox(width))
	}
	return b.String()
}

func updateScreenLayout(m CLIModel) CLIModel {
	if m.width == 0 || m.height == 0 {
		return m
	}

	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	headerHeight := lipgloss.Height(m.renderHeader(contentWidth))
	footerHeight := lipgloss.Height(m.renderFooter(contentWidth))

	vpHeight := m.height - headerHeight - footerHeight - 4 // padding buffer
	if vpHeight < 5 {
		vpHeight = 5 // fail-safe bounds
	}

	if !m.ready {
		m.viewport = viewport.New(contentWidth, vpHeight)
		m.viewport.YPosition = headerHeight + 1
		m.viewport.HighPerformanceRendering = false
		m.ready = true
	} else {
		m.viewport.Width = contentWidth
		m.viewport.Height = vpHeight
	}

	// Always sync contents
	content := m.renderHistoryToString(contentWidth)
	m.viewport.SetContent(content)
	m.viewport.GotoBottom() // Automagically scroll to new chats
	
	return m
}

func (m CLIModel) View() string {
	if !m.ready {
		return "\n  Iniciando painel..."
	}

	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	header := m.renderHeader(contentWidth)
	footer := m.renderFooter(contentWidth)

	uiView := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"\n"+m.viewport.View(),
		footer,
	)

	return m.styles.App.Render(uiView)
}

// wrapText adds dynamic word-wrapping functionality
func (m CLIModel) wrapText(text string, maxWidth int) string {
	if lipgloss.Width(text) > maxWidth {
		return lipgloss.NewStyle().Width(maxWidth).Render(text)
	}
	return text
}

func (m CLIModel) renderHistoryToString(width int) string {
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
			
			gr, err := glamour.NewTermRenderer(
				glamour.WithWordWrap(maxBubbleWidth-4),
				glamour.WithStandardStyle("dark"),
			)
			
			mdContent := msg.Content
			if err == nil {
				if parsed, err := gr.Render(msg.Content); err == nil {
					mdContent = strings.TrimSpace(parsed)
				} else {
					mdContent = m.wrapText(msg.Content, maxBubbleWidth)
				}
			} else {
				mdContent = m.wrapText(msg.Content, maxBubbleWidth)
			}
			
			bubble := m.styles.QueenBubble.Render(mdContent)
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
	header := fmt.Sprintf("%s ⚠️  %s: %s",
		m.Spinner.View(),
		i18n.T("cli_request_approval"),
		m.styles.WarningLabel.Render(m.ApprovalTool),
	)

	argsBlock := m.formatApprovalArgs(m.ApprovalArgs)
	prompt := m.styles.ApprovalPrompt.Render(i18n.T("cli_approval_prompt"))

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		argsBlock,
		prompt,
	)

	return m.styles.ApprovalBox.Width(width).Render(content) + "\n"
}

func (m CLIModel) formatApprovalArgs(rawJSON string) string {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		if len(rawJSON) > 150 {
			rawJSON = rawJSON[:150] + " ... [TRUNCADO]"
		}
		return lipgloss.NewStyle().Foreground(DefaultTheme.Text).Italic(true).Render(rawJSON)
	}

	var lines []string
	codeBg := lipgloss.NewStyle().Background(DefaultTheme.BgDark).Padding(0, 1)

	for k, v := range parsed {
		strVal := fmt.Sprintf("%v", v)
		
		// Truncar textos ou códigos muito grandes para não estourar a tela do terminal
		// e causar o sumiço do Header/Sumário por causa do scroll natural.
		splitLines := strings.Split(strVal, "\n")
		if len(splitLines) > 5 {
			strVal = strings.Join(splitLines[:5], "\n") + fmt.Sprintf("\n... [+%d linhas ocultas para economizar espaço]", len(splitLines)-5)
		} else if len(strVal) > 300 {
			strVal = strVal[:300] + " ... [TRUNCADO]"
		}

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
	inputContent := m.styles.InputLabel.Render(i18n.T("cli_prompt_goal")+" ") + m.TextInput.View()
	box := m.styles.InputBox.Width(width).Render(inputContent)
	tips := m.styles.Footer.Width(width).Align(lipgloss.Center).Render(i18n.T("cli_footer"))
	return lipgloss.JoinVertical(lipgloss.Left, box, tips) + "\n"
}

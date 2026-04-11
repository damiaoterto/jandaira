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
	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/service"
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
	BgDeep    lipgloss.Color
	Border    lipgloss.Color
	BorderDim lipgloss.Color
}

var DefaultTheme = Theme{
	Primary:   lipgloss.Color("#D4A847"), // Amber dourado
	Secondary: lipgloss.Color("#E8C96A"), // Amber claro
	Accent:    lipgloss.Color("#8AB4E8"), // Azul suave
	Success:   lipgloss.Color("#7AC040"), // Verde
	Info:      lipgloss.Color("#4A9EFF"), // Azul info
	Warning:   lipgloss.Color("#D4A847"), // Amber warning
	Error:     lipgloss.Color("#C04040"), // Vermelho
	Text:      lipgloss.Color("#C9CDD6"), // Texto principal
	Muted:     lipgloss.Color("#4A5066"), // Texto muted
	Surface:   lipgloss.Color("#13151D"), // Superfície de cards
	BgDark:    lipgloss.Color("#0D0F14"), // Background principal
	BgDeep:    lipgloss.Color("#0A0C12"), // Background mais escuro
	Border:    lipgloss.Color("#2A2D3E"), // Borda padrão
	BorderDim: lipgloss.Color("#1E2130"), // Borda sutil
}

type Styles struct {
	App lipgloss.Style

	// Header
	TitleBar  lipgloss.Style
	TitleText lipgloss.Style
	TitleSub  lipgloss.Style
	BadgeGo   lipgloss.Style
	BadgeVer  lipgloss.Style

	// Tabs
	TabBar    lipgloss.Style
	TabActive lipgloss.Style
	TabInact  lipgloss.Style

	// History
	SystemMessage lipgloss.Style
	UserBubble    lipgloss.Style
	QueenBubble   lipgloss.Style
	ErrorBubble   lipgloss.Style
	QueenHeader   lipgloss.Style
	ErrorHeader   lipgloss.Style

	// Swarm Plan
	PlanBox    lipgloss.Style
	PlanLabel  lipgloss.Style
	PlanCount  lipgloss.Style
	AgentCard  lipgloss.Style
	AgentName  lipgloss.Style
	AgentRole  lipgloss.Style
	ToolPill   lipgloss.Style
	StatusRun  lipgloss.Style
	StatusWait lipgloss.Style

	// Approval
	ApprovalBox   lipgloss.Style
	ApprovalTitle lipgloss.Style
	ApprovalTool  lipgloss.Style
	ApprovalArgs  lipgloss.Style
	ApprovalKey   lipgloss.Style
	ApprovalValue lipgloss.Style
	BtnAllow      lipgloss.Style
	BtnDeny       lipgloss.Style
	ApprovalHint  lipgloss.Style

	// Log
	LogBox  lipgloss.Style
	LogTs   lipgloss.Style
	LogInfo lipgloss.Style
	LogPlan lipgloss.Style
	LogWait lipgloss.Style
	LogMsg  lipgloss.Style
	LogHL   lipgloss.Style

	// Wizard (compatibilidade)
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Status   lipgloss.Style

	// Input
	InputBar   lipgloss.Style
	InputBox   lipgloss.Style
	InputLabel lipgloss.Style
	Footer     lipgloss.Style

	// Nectar bar
	NectarBar   lipgloss.Style
	NectarLabel lipgloss.Style
	NectarVal   lipgloss.Style
	NectarMeta  lipgloss.Style
}

func DefaultStyles() *Styles {
	t := DefaultTheme

	return &Styles{
		App: lipgloss.NewStyle().Padding(0),

		// ── Header ──────────────────────────────────────────────────
		TitleBar: lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(t.BorderDim).
			Padding(0, 2),

		TitleText: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		TitleSub: lipgloss.NewStyle().
			Foreground(t.Muted),

		BadgeGo: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4A9EFF")).
			Background(lipgloss.Color("#1A2535")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#1E3A5C")).
			Padding(0, 1),

		BadgeVer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7AC040")).
			Background(lipgloss.Color("#1A2018")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#2A4020")).
			Padding(0, 1),

		// ── Tabs ────────────────────────────────────────────────────
		TabBar: lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(t.BorderDim),

		TabActive: lipgloss.NewStyle().
			Foreground(t.Primary).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(t.Primary).
			Padding(0, 2),

		TabInact: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3D4158")).
			Padding(0, 2),

		// ── History ─────────────────────────────────────────────────
		SystemMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3D4158")).
			Italic(true).
			MarginBottom(1),

		UserBubble: lipgloss.NewStyle().
			Foreground(t.Accent).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#1E3A5C")).
			Padding(0, 1).
			MarginBottom(1).
			MarginTop(1),

		QueenBubble: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(t.BorderDim).
			BorderLeft(true).
			BorderLeftForeground(t.Primary).
			Padding(0, 1).
			MarginBottom(1),

		ErrorBubble: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#502020")).
			BorderLeft(true).
			BorderLeftForeground(t.Error).
			Foreground(t.Error).
			Padding(0, 1).
			MarginBottom(1),

		QueenHeader: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			MarginBottom(1),

		ErrorHeader: lipgloss.NewStyle().
			Foreground(t.Error).
			Bold(true).
			MarginBottom(1),

		// ── Swarm Plan ───────────────────────────────────────────────
		PlanBox: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(t.BorderDim).
			BorderLeft(true).
			BorderLeftForeground(t.Primary).
			Padding(1, 2).
			MarginBottom(1),

		PlanLabel: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		PlanCount: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8AB040")).
			Background(lipgloss.Color("#1E200A")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#3A4A10")).
			Padding(0, 1),

		AgentCard: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(t.Border).
			Padding(0, 1).
			MarginTop(1),

		AgentName: lipgloss.NewStyle().
			Foreground(t.Secondary).
			Bold(true),

		AgentRole: lipgloss.NewStyle().
			Foreground(t.Muted),

		ToolPill: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5A7AAA")).
			Background(lipgloss.Color("#131825")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#1E2A3E")).
			Padding(0, 1),

		StatusRun: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4A9EFF")).
			Background(lipgloss.Color("#0A1E2E")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#1A3A5C")).
			Padding(0, 1),

		StatusWait: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6A8A30")).
			Background(lipgloss.Color("#1A1F10")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#2A3A10")).
			Padding(0, 1),

		// ── Approval ────────────────────────────────────────────────
		ApprovalBox: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#2E1F0A")).
			BorderLeft(true).
			BorderLeftForeground(t.Primary).
			Padding(1, 2).
			MarginBottom(1),

		ApprovalTitle: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		ApprovalTool: lipgloss.NewStyle().
			Foreground(t.Secondary).
			Background(lipgloss.Color("#1A120A")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#3A2A10")).
			Bold(true).
			Padding(0, 1),

		ApprovalArgs: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(t.BorderDim).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1),

		ApprovalKey: lipgloss.NewStyle().
			Foreground(t.Muted),

		ApprovalValue: lipgloss.NewStyle().
			Foreground(t.Accent),

		BtnAllow: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7AC040")).
			Background(lipgloss.Color("#1A2810")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#3A5820")).
			Bold(true).
			Padding(0, 2),

		BtnDeny: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C04040")).
			Background(lipgloss.Color("#1E1010")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#502020")).
			Bold(true).
			Padding(0, 2),

		ApprovalHint: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2E3248")).
			Italic(true),

		// ── Log ─────────────────────────────────────────────────────
		LogBox: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(t.BorderDim).
			Padding(0, 1).
			MarginBottom(1),

		LogTs: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2E3248")),

		LogInfo: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4A9EFF")).
			Bold(true),

		LogPlan: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7AC040")).
			Bold(true),

		LogWait: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		LogMsg: lipgloss.NewStyle().
			Foreground(t.Muted),

		LogHL: lipgloss.NewStyle().
			Foreground(t.Accent),

		// ── Input ───────────────────────────────────────────────────
		InputBar: lipgloss.NewStyle().
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(t.BorderDim).
			Padding(0, 2),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),

		InputLabel: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2E3248")).
			Italic(true),

		// ── Nectar ──────────────────────────────────────────────────
		NectarBar: lipgloss.NewStyle().
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(t.BorderDim).
			Padding(0, 2),

		NectarLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3D4158")).
			Bold(true),

		NectarVal: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		NectarMeta: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3D4158")),

		// ── Wizard (compatibilidade) ─────────────────────────────────
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
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Messages
// ─────────────────────────────────────────────────────────────────────────────

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
	Role    string
	Content string
}

type LogEntry struct {
	Timestamp string
	Level     string // "INFO", "PLAN", "WAIT", "ERR"
	Message   string
	Highlight string
}

// ─────────────────────────────────────────────────────────────────────────────
// Model
// ─────────────────────────────────────────────────────────────────────────────

type CLIModel struct {
	Queen            *swarm.Queen
	SwarmName        string
	TextInput        textinput.Model
	Spinner          spinner.Model
	viewport         viewport.Model
	History          []HistoryMsg
	Logs             []LogEntry
	ActivePipeline   []swarm.Specialist
	IsWorking        bool
	StatusLine       string
	WaitingApproval  bool
	ApprovalTool     string
	ApprovalArgs     string
	PlanSummary      string
	NectarUsed       int
	NectarMax        int
	width            int
	height           int
	ready            bool
	styles           *Styles
	maxWorkers       int
	sessionService   service.SessionService
	currentSessionID string
}

func InitialModel(q *swarm.Queen, swarmName string, maxWorkers int, sessionSvc service.SessionService) CLIModel {
	styles := DefaultStyles()

	ti := textinput.New()
	ti.Placeholder = i18n.T("cli_prompt_placeholder")
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 52
	ti.PromptStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Primary)
	ti.TextStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Text)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(DefaultTheme.Primary)

	return CLIModel{
		Queen:          q,
		SwarmName:      swarmName,
		TextInput:      ti,
		Spinner:        s,
		History:        []HistoryMsg{{Role: "system", Content: i18n.T("cli_greeting")}},
		Logs:           []LogEntry{},
		IsWorking:      false,
		styles:         styles,
		maxWorkers:     maxWorkers,
		NectarMax:      50000,
		sessionService: sessionSvc,
	}
}

func (m *CLIModel) addLog(level, message, highlight string) {
	m.Logs = append(m.Logs, LogEntry{
		Timestamp: time.Now().Format("15:04:05"),
		Level:     level,
		Message:   message,
		Highlight: highlight,
	})
	// Manter apenas os últimos 6 logs visíveis
	if len(m.Logs) > 6 {
		m.Logs = m.Logs[len(m.Logs)-6:]
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Init / Update
// ─────────────────────────────────────────────────────────────────────────────

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
		if m.width > 16 {
			m.TextInput.Width = m.width - 16
		}
		return m, nil

	case tea.KeyMsg:
		if m.WaitingApproval {
			switch msg.String() {
			case "s", "S", "y", "Y":
				m.WaitingApproval = false
				m.PlanSummary = ""
				m.Queen.ApprovalChan <- true
				m.addLog("INFO", "permissão concedida →", m.ApprovalTool)
				m.StatusLine = "Rainha retomou a tarefa..."
				return m, nil
			case "n", "N":
				m.WaitingApproval = false
				m.PlanSummary = ""
				m.Queen.ApprovalChan <- false
				m.addLog("WAIT", "ação bloqueada →", m.ApprovalTool)
				m.StatusLine = "Rainha está reconsiderando..."
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
			m.addLog("INFO", "rainha recebeu objetivo →", truncate(goal, 40))
			cmds = append(cmds, m.runGoal(goal))
		}

	case PermissionMsg:
		m.WaitingApproval = true
		m.ApprovalTool = msg.ToolName
		m.ApprovalArgs = msg.Args
		m.addLog("WAIT", "aguardando aprovação →", msg.ToolName)
		return m, nil

	case PlanReadyMsg:
		m.ActivePipeline = msg.pipeline
		m.addLog("PLAN", fmt.Sprintf("enxame montado com %d operária(s)", len(msg.pipeline)), "")

		// Persist session + agents in the background (SQLite is fast locally).
		if m.sessionService != nil {
			session, err := m.sessionService.Create("", msg.goal)
			if err == nil {
				m.currentSessionID = session.ID
				for _, spec := range msg.pipeline {
					_, _ = m.sessionService.AddAgent(session.ID, spec.Name, "specialist")
				}
				m.addLog("INFO", fmt.Sprintf("sessão %s criada", session.ID[:8]), "")
				// Update AgentChangeFunc to track active agent in the session.
				sessionID := session.ID
				m.Queen.AgentChangeFunc = func(agentName string) {
					_ = m.sessionService.UpdateAgentStatusByName(sessionID, agentName, model.AgentStatusWorking)
				}
			}
		}

		var summary strings.Builder
		summary.WriteString(fmt.Sprintf("🐝 Enxame planejado com %d operária(s):\n", len(msg.pipeline)))
		for _, spec := range msg.pipeline {
			roleSnippet := spec.SystemPrompt
			if len(roleSnippet) > 100 {
				roleSnippet = roleSnippet[:97] + "..."
			}
			summary.WriteString(fmt.Sprintf("\n▸ %s\n  Papel: %s\n  Ferramentas: %s\n",
				spec.Name, roleSnippet, strings.Join(spec.AllowedTools, ", ")))
		}
		m.PlanSummary = summary.String()
		cmds = append(cmds, m.executePipeline(msg.goal, msg.pipeline))

	case ResultMsg:
		m.IsWorking = false
		m.ActivePipeline = nil
		m.PlanSummary = ""
		if msg.err != nil {
			m.History = append(m.History, HistoryMsg{Role: "error", Content: msg.err.Error()})
			m.addLog("ERR", "missão falhou →", msg.err.Error())
			if m.sessionService != nil && m.currentSessionID != "" {
				_ = m.sessionService.FailSession(m.currentSessionID)
			}
		} else {
			m.History = append(m.History, HistoryMsg{Role: "queen", Content: msg.content})
			m.addLog("INFO", "missão concluída", "")
			if m.sessionService != nil && m.currentSessionID != "" {
				_ = m.sessionService.CompleteSession(m.currentSessionID, msg.content)
			}
		}
		m.currentSessionID = ""
		m.StatusLine = ""

	case StatusMsg:
		m.StatusLine = string(msg)

	case spinner.TickMsg:
		m.Spinner, cmd = m.Spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.TextInput, cmd = m.TextInput.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	m = updateScreenLayout(m)

	return m, tea.Batch(cmds...)
}

// ─────────────────────────────────────────────────────────────────────────────
// Commands
// ─────────────────────────────────────────────────────────────────────────────

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

// ─────────────────────────────────────────────────────────────────────────────
// Layout
// ─────────────────────────────────────────────────────────────────────────────

func updateScreenLayout(m CLIModel) CLIModel {
	if m.width == 0 || m.height == 0 {
		return m
	}
	contentWidth := m.width
	if contentWidth < 40 {
		contentWidth = 40
	}

	headerH := lipgloss.Height(m.renderHeader(contentWidth))
	tabsH := lipgloss.Height(m.renderTabs(contentWidth))
	footerH := lipgloss.Height(m.renderFooter(contentWidth))
	nectarH := lipgloss.Height(m.renderNectarBar(contentWidth))

	vpHeight := m.height - headerH - tabsH - footerH - nectarH - 1
	if vpHeight < 5 {
		vpHeight = 5
	}

	if !m.ready {
		m.viewport = viewport.New(contentWidth, vpHeight)
		m.viewport.HighPerformanceRendering = false
		m.ready = true
	} else {
		m.viewport.Width = contentWidth
		m.viewport.Height = vpHeight
	}

	m.viewport.SetContent(m.renderHistoryToString(contentWidth))
	m.viewport.GotoBottom()
	return m
}

// ─────────────────────────────────────────────────────────────────────────────
// View
// ─────────────────────────────────────────────────────────────────────────────

func (m CLIModel) View() string {
	if !m.ready {
		return "\n  Iniciando colmeia..."
	}
	w := m.width

	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(w),
		m.renderTabs(w),
		m.viewport.View(),
		m.renderFooter(w),
		m.renderNectarBar(w),
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Render: Header
// ─────────────────────────────────────────────────────────────────────────────

func (m CLIModel) renderHeader(width int) string {
	s := m.styles
	t := DefaultTheme

	dots := lipgloss.NewStyle().Render(
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F57")).Render("●") + " " +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#FEBC2E")).Render("●") + " " +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#28C840")).Render("●"),
	)

	titleCenter := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Foreground(t.Primary).Render("🍯  "),
		s.TitleText.Render("JANDAIRA SWARM OS"),
		lipgloss.NewStyle().Foreground(t.Muted).Render("  ·  Swarm Intelligence · Powered by Go"),
	)

	badges := lipgloss.JoinHorizontal(lipgloss.Center,
		s.BadgeGo.Render("Go 1.22"),
		lipgloss.NewStyle().Render(" "),
		s.BadgeVer.Render("v1.2.0"),
	)

	// Calcular larguras para posicionamento
	dotsW := lipgloss.Width(dots)
	badgesW := lipgloss.Width(badges)
	innerW := width - 4 // padding

	centerW := innerW - dotsW - badgesW
	if centerW < 10 {
		centerW = 10
	}

	centered := lipgloss.PlaceHorizontal(centerW, lipgloss.Center, titleCenter)
	row := lipgloss.JoinHorizontal(lipgloss.Center, dots, centered, badges)

	return s.TitleBar.Width(width).Render(row)
}

// ─────────────────────────────────────────────────────────────────────────────
// Render: Tabs
// ─────────────────────────────────────────────────────────────────────────────

func (m CLIModel) renderTabs(width int) string {
	s := m.styles
	tabs := lipgloss.JoinHorizontal(lipgloss.Left,
		s.TabActive.Render("Missão"),
		s.TabInact.Render("Memória"),
		s.TabInact.Render("Ferramentas"),
		s.TabInact.Render("Néctar"),
	)
	return s.TabBar.Width(width).Render(tabs)
}

// ─────────────────────────────────────────────────────────────────────────────
// Render: Nectar bar
// ─────────────────────────────────────────────────────────────────────────────

func (m CLIModel) renderNectarBar(width int) string {
	s := m.styles

	pct := 0
	if m.NectarMax > 0 {
		pct = (m.NectarUsed * 20) / m.NectarMax
	}
	if pct > 20 {
		pct = 20
	}

	track := strings.Repeat("━", pct) + lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1E2130")).
		Render(strings.Repeat("━", 20-pct))

	nectarUsedK := fmt.Sprintf("%.1fk", float64(m.NectarUsed)/1000)

	row := lipgloss.JoinHorizontal(lipgloss.Center,
		s.NectarLabel.Render("NÉCTAR"),
		lipgloss.NewStyle().Render("  "),
		lipgloss.NewStyle().Foreground(DefaultTheme.Primary).Render(track),
		lipgloss.NewStyle().Render("  "),
		s.NectarVal.Render(nectarUsedK),
		s.NectarMeta.Render(fmt.Sprintf(" / %dk tokens", m.NectarMax/1000)),
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3D4158")).
			Render(fmt.Sprintf("   %s · isolado · HIL ativo", m.SwarmName)),
	)

	return s.NectarBar.Width(width).Render(row)
}

// ─────────────────────────────────────────────────────────────────────────────
// Render: Footer (plano + aprovação + input)
// ─────────────────────────────────────────────────────────────────────────────

func (m CLIModel) renderFooter(width int) string {
	var parts []string

	// Swarm plan
	if len(m.ActivePipeline) > 0 {
		parts = append(parts, m.renderPlanBox(width))
	}

	// Approval
	if m.WaitingApproval {
		parts = append(parts, m.renderApprovalBox(width))
	}

	// Log
	if len(m.Logs) > 0 {
		parts = append(parts, m.renderLogBox(width))
	}

	// Input ou spinner
	if !m.WaitingApproval {
		if m.IsWorking {
			spinLine := fmt.Sprintf("  %s  %s",
				m.Spinner.View(),
				lipgloss.NewStyle().Foreground(DefaultTheme.Primary).Italic(true).Render(m.StatusLine),
			)
			parts = append(parts, m.styles.InputBar.Width(width).Render(spinLine))
		} else {
			parts = append(parts, m.renderInputBar(width))
		}
	} else {
		parts = append(parts, m.styles.InputBar.Width(width).Render(
			lipgloss.NewStyle().Foreground(lipgloss.Color("#2E3248")).Italic(true).Render("  aguardando decisão do Apicultor..."),
		))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// ─────────────────────────────────────────────────────────────────────────────
// Render: Plan Box
// ─────────────────────────────────────────────────────────────────────────────

func (m CLIModel) renderPlanBox(width int) string {
	s := m.styles

	header := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Render("🐝  "),
		s.PlanLabel.Render("ENXAME PLANEJADO"),
		lipgloss.NewStyle().Render("  "),
		s.PlanCount.Render(fmt.Sprintf("%d operária(s)", len(m.ActivePipeline))),
	)

	var agentCards []string
	agentCards = append(agentCards, header)

	for i, spec := range m.ActivePipeline {
		statusBadge := s.StatusWait.Render("aguardando")
		if i == 0 && m.IsWorking {
			statusBadge = s.StatusRun.Render("executando")
		}

		nameRow := lipgloss.JoinHorizontal(lipgloss.Center,
			s.AgentName.Render("▶  "+spec.Name),
			lipgloss.NewStyle().Render("  "),
			statusBadge,
		)

		roleText := spec.SystemPrompt
		if len(roleText) > 80 {
			roleText = roleText[:77] + "..."
		}
		role := s.AgentRole.Render(roleText)

		var pills []string
		for _, tool := range spec.AllowedTools {
			pills = append(pills, s.ToolPill.Render(tool))
		}
		toolsRow := strings.Join(pills, " ")

		cardContent := lipgloss.JoinVertical(lipgloss.Left, nameRow, role, toolsRow)
		agentCards = append(agentCards, s.AgentCard.Width(width-8).Render(cardContent))
	}

	body := lipgloss.JoinVertical(lipgloss.Left, agentCards...)
	return s.PlanBox.Width(width).Render(body)
}

// ─────────────────────────────────────────────────────────────────────────────
// Render: Approval Box
// ─────────────────────────────────────────────────────────────────────────────

func (m CLIModel) renderApprovalBox(width int) string {
	s := m.styles

	titleRow := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Foreground(DefaultTheme.Warning).Render("⚠  "),
		s.ApprovalTitle.Render("APROVAÇÃO NECESSÁRIA  ·  APICULTOR"),
	)

	toolRow := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Foreground(DefaultTheme.Muted).Render("ferramenta  "),
		s.ApprovalTool.Render(m.ApprovalTool),
	)

	argsBlock := m.formatApprovalArgs(m.ApprovalArgs, width-12)

	btnRow := lipgloss.JoinHorizontal(lipgloss.Center,
		s.BtnAllow.Render("S — Autorizar"),
		lipgloss.NewStyle().Render("  "),
		s.BtnDeny.Render("N — Bloquear"),
		lipgloss.NewStyle().Render("  "),
		s.ApprovalHint.Render("a IA aguarda sua decisão"),
	)

	body := lipgloss.JoinVertical(lipgloss.Left,
		titleRow, "",
		toolRow, "",
		argsBlock,
		btnRow,
	)

	return s.ApprovalBox.Width(width).Render(body)
}

func (m CLIModel) formatApprovalArgs(rawJSON string, maxWidth int) string {
	s := m.styles
	var parsed map[string]interface{}

	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		content := rawJSON
		if len(content) > 200 {
			content = content[:200] + " [TRUNCADO]"
		}
		return s.ApprovalArgs.Width(maxWidth).Render(
			s.ApprovalValue.Render(content),
		)
	}

	var lines []string
	for k, v := range parsed {
		strVal := fmt.Sprintf("%v", v)
		splitLines := strings.Split(strVal, "\n")
		if len(splitLines) > 5 {
			strVal = strings.Join(splitLines[:5], "\n") + fmt.Sprintf("\n[+%d linhas ocultas]", len(splitLines)-5)
		} else if len(strVal) > 300 {
			strVal = strVal[:300] + " [TRUNCADO]"
		}
		lines = append(lines,
			s.ApprovalKey.Render(k+"  ")+s.ApprovalValue.Render(strVal),
		)
	}
	return s.ApprovalArgs.Width(maxWidth).Render(strings.Join(lines, "\n"))
}

// ─────────────────────────────────────────────────────────────────────────────
// Render: Log Box
// ─────────────────────────────────────────────────────────────────────────────

func (m CLIModel) renderLogBox(width int) string {
	s := m.styles
	var lines []string

	for _, entry := range m.Logs {
		levelStyle := s.LogInfo
		switch entry.Level {
		case "PLAN":
			levelStyle = s.LogPlan
		case "WAIT":
			levelStyle = s.LogWait
		case "ERR":
			levelStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Error).Bold(true)
		}

		row := lipgloss.JoinHorizontal(lipgloss.Left,
			s.LogTs.Render(entry.Timestamp+"  "),
			levelStyle.Render(fmt.Sprintf("%-4s", entry.Level)+"  "),
			s.LogMsg.Render(entry.Message+"  "),
			s.LogHL.Render(entry.Highlight),
		)
		lines = append(lines, row)
	}

	return s.LogBox.Width(width).Render(strings.Join(lines, "\n"))
}

// ─────────────────────────────────────────────────────────────────────────────
// Render: Input Bar
// ─────────────────────────────────────────────────────────────────────────────

func (m CLIModel) renderInputBar(width int) string {
	s := m.styles

	prefix := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Render("🐝  "),
		s.InputLabel.Render("Objetivo"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#2E3248")).Render("  ›  "),
	)

	hints := s.Footer.Render("↵ enviar   esc sair")

	prefixW := lipgloss.Width(prefix)
	hintsW := lipgloss.Width(hints)
	inputW := width - prefixW - hintsW - 8
	if inputW < 10 {
		inputW = 10
	}
	m.TextInput.Width = inputW

	row := lipgloss.JoinHorizontal(lipgloss.Center,
		prefix,
		m.TextInput.View(),
		lipgloss.PlaceHorizontal(hintsW+4, lipgloss.Right, hints),
	)

	return s.InputBar.Width(width).Render(row)
}

// ─────────────────────────────────────────────────────────────────────────────
// Render: History
// ─────────────────────────────────────────────────────────────────────────────

func (m CLIModel) renderHistoryToString(width int) string {
	var b strings.Builder
	s := m.styles

	maxBubbleWidth := int(float64(width) * 0.82)
	if maxBubbleWidth < 30 {
		maxBubbleWidth = width
	}

	for _, msg := range m.History {
		b.WriteString("\n")
		switch msg.Role {
		case "system":
			row := lipgloss.PlaceHorizontal(width, lipgloss.Left,
				lipgloss.NewStyle().Foreground(lipgloss.Color("#3D4158")).Italic(true).
					Render("  ✦  "+msg.Content),
			)
			b.WriteString(row)

		case "user":
			content := lipgloss.NewStyle().Width(maxBubbleWidth).Render(msg.Content)
			bubble := s.UserBubble.Render(content)
			row := lipgloss.PlaceHorizontal(width, lipgloss.Right, bubble)
			b.WriteString(row)

		case "queen":
			header := s.QueenHeader.Render("👑  Rainha")

			gr, err := glamour.NewTermRenderer(
				glamour.WithWordWrap(maxBubbleWidth-4),
				glamour.WithStandardStyle("dark"),
			)
			mdContent := msg.Content
			if err == nil {
				if parsed, parseErr := gr.Render(msg.Content); parseErr == nil {
					mdContent = strings.TrimSpace(parsed)
				}
			}

			bubble := s.QueenBubble.Width(maxBubbleWidth).Render(mdContent)
			body := lipgloss.JoinVertical(lipgloss.Left, header, bubble)
			b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Left, body))

		case "error":
			header := s.ErrorHeader.Render("⚠  Erro")
			content := lipgloss.NewStyle().Width(maxBubbleWidth).Render(msg.Content)
			bubble := s.ErrorBubble.Width(maxBubbleWidth).Render(content)
			body := lipgloss.JoinVertical(lipgloss.Left, header, bubble)
			b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Left, body))
		}
	}
	return b.String()
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

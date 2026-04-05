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

// ── Palette ─────────────────────────────────────────────────────────────────
const (
	colorGold        = "#F5A623"
	colorAmber       = "#FFD166"
	colorQueenPink   = "#F78FA7"
	colorWorkerGreen = "#06D6A0"
	colorUserCyan    = "#48CAE4"
	colorErrorRed    = "#EF476F"
	colorMuted       = "#6C6C8A"
	colorBorder      = "#2D2B55"
	colorWarning     = "#FF8C00"
)

// ── Base styles ─────────────────────────────────────────────────────────────
var (
	// Banner box
	bannerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color(colorGold)).
			Padding(0, 3).
			MarginBottom(1)

	bannerTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorAmber)).
				Bold(true)

	bannerSubStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)).
			Italic(true)

	// Message bubbles
	userLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorUserCyan)).
			Bold(true)

	userMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0F4FF")).
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(colorUserCyan)).
			PaddingLeft(1)

	queenLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorQueenPink)).
			Bold(true)

	queenMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF0F5")).
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(colorQueenPink)).
			PaddingLeft(1)

	errorLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorErrorRed)).
			Bold(true)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFE0E6")).
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(colorErrorRed)).
			PaddingLeft(1)

	// Divider
	dividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorBorder))

	// Status / spinner line
	StatusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAmber)).
			Italic(true)

	// Input box
	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorGold)).
			Padding(0, 1)

	inputLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAmber)).
			Bold(true)

	// Footer hint
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)).
			Italic(true)

	// Apicultor (beekeeper) approval panel
	approvalBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(colorWarning)).
			Padding(1, 2).
			MarginTop(1)

	warningLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorWarning)).
			Bold(true)

	approvalArgsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D4C5A9")).
			Italic(true)

	approvalPromptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorWarning)).
			Bold(true)

	// Legacy aliases kept so nothing outside this file breaks
	TitleStyle  = bannerTitleStyle
	QueenStyle  = queenLabelStyle
	WorkerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorWorkerGreen))
	ErrorStyle  = errorLabelStyle
	SystemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Italic(true)
	InputPrompt = inputLabelStyle.Render("🐝 Objetivo: ")
)

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

type CLIModel struct {
	Queen      *swarm.Queen
	Pipeline   []swarm.Specialist
	TextInput  textinput.Model
	Spinner    spinner.Model
	History    []string
	IsWorking  bool
	StatusLine string

	// Apicultor (beekeeper) Human-in-the-Loop state
	WaitingApproval bool
	ApprovalTool    string
	ApprovalArgs    string
}

// divider returns a horizontal rule string.
func divider() string {
	return dividerStyle.Render(strings.Repeat("─", 58))
}

// formatApprovalArgs turns raw tool-call JSON into a human-friendly block.
// Each key is highlighted; multi-line values (e.g. file content) are indented
// and shown in a dimmer colour so the beekeeper can read them at a glance.
func formatApprovalArgs(rawJSON string) string {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		// Fallback: show the raw string as-is
		return approvalArgsStyle.Render(rawJSON)
	}

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorAmber)).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D4C5A9"))

	codeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A8D8A8")).
		Background(lipgloss.Color("#1A1A2E")).
		Padding(0, 1)

	var lines []string
	for k, v := range parsed {
		strVal := fmt.Sprintf("%v", v)
		if strings.Contains(strVal, "\n") {
			// Multi-line value → render as a code block
			indented := strings.ReplaceAll(strVal, "\n", "\n  ")
			lines = append(lines,
				keyStyle.Render("▸ "+k+":"),
				codeStyle.Render("  "+indented),
			)
		} else {
			lines = append(lines,
				keyStyle.Render("▸ "+k+": ")+valueStyle.Render(strVal),
			)
		}
	}
	return strings.Join(lines, "\n")
}

// View implements [tea.Model].
func (m CLIModel) View() string {
	var b strings.Builder

	// ── Banner ──────────────────────────────────────────────
	banner := lipgloss.JoinVertical(
		lipgloss.Center,
		bannerTitleStyle.Render("🍯  Jandaira Swarm OS  🍯"),
		bannerSubStyle.Render("Swarm Intelligence · Powered by Go"),
	)
	b.WriteString(bannerBoxStyle.Render(banner))
	b.WriteString("\n")

	// ── History ─────────────────────────────────────────────
	for _, msg := range m.History {
		b.WriteString(msg)
		b.WriteString("\n")
		b.WriteString(divider())
		b.WriteString("\n")
	}

	// ── Apicultor approval panel ─────────────────────────────
	if m.WaitingApproval {
		header := fmt.Sprintf("%s  ⚠️  A IA quer usar a ferramenta  %s",
			m.Spinner.View(),
			warningLabelStyle.Render("'"+m.ApprovalTool+"'"),
		)
		argsBlock := formatApprovalArgs(m.ApprovalArgs)
		promptLine := approvalPromptStyle.Render("👨\u200d🌾 Você autoriza? (S = sim / N = não)")

		panelContent := lipgloss.JoinVertical(lipgloss.Left,
			header,
			"",
			argsBlock,
			"",
			promptLine,
		)
		b.WriteString(approvalBoxStyle.Render(panelContent) + "\n")

	// ── Spinner or Input ────────────────────────────────────
	} else if m.IsWorking {
		spinnerLine := fmt.Sprintf("%s  %s", m.Spinner.View(), StatusStyle.Render(m.StatusLine))
		b.WriteString("\n" + spinnerLine + "\n")
	} else {
		inputContent := inputLabelStyle.Render("🐝 Objetivo") + "  " + m.TextInput.View()
		b.WriteString("\n" + inputBoxStyle.Render(inputContent) + "\n")
		b.WriteString(footerStyle.Render("  ↵ enviar   esc / ctrl+c sair") + "\n")
	}

	return b.String()
}

func InitialModel(q *swarm.Queen, p []swarm.Specialist) CLIModel {
	ti := textinput.New()
	ti.Placeholder = "Diga à Rainha o que deseja fazer..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 52
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGold))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorAmber))

	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGold))

	welcome := SystemStyle.Render("✦ A Colmeia Jandaira despertou. As operárias aguardam as suas ordens.")

	return CLIModel{
		Queen:     q,
		Pipeline:  p,
		TextInput: ti,
		Spinner:   s,
		History:   []string{welcome},
		IsWorking: false,
	}
}

func (m CLIModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.Spinner.Tick)
}

func (m CLIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
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
			userBubble := userLabelStyle.Render("👤 Você") + "\n" + userMsgStyle.Render(goal)
			m.History = append(m.History, userBubble)
			m.IsWorking = true
			m.StatusLine = "A Rainha está a analisar a tarefa..."
			cmds = append(cmds, m.runGoal(goal))
		}

	// ── Apicultor permission request from the Queen goroutine ──
	case PermissionMsg:
		m.WaitingApproval = true
		m.ApprovalTool = msg.ToolName
		m.ApprovalArgs = msg.Args
		return m, nil

	case ResultMsg:
		m.IsWorking = false
		if msg.err != nil {
			errBubble := errorLabelStyle.Render("⚠  Erro") + "\n" + errorMsgStyle.Render(fmt.Sprintf("%v", msg.err))
			m.History = append(m.History, errBubble)
		} else {
			queenBubble := queenLabelStyle.Render("👑 Rainha") + "\n" + queenMsgStyle.Render(msg.content)
			m.History = append(m.History, queenBubble)
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

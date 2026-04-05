package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/damiaoterto/jandaira/internal/swarm"
)

var (
	TitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")).Bold(true).MarginBottom(1)
	QueenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF69B4")).Bold(true)
	WorkerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	ErrorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	SystemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Italic(true)
	InputPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")).Render("🐝 Objetivo: ")
)

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
}

// View implements [tea.Model].
func (m CLIModel) View() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("\n🍯 Jandaira Swarm OS 🍯\n"))

	for _, msg := range m.History {
		b.WriteString(msg + "\n\n")
	}

	if m.IsWorking {
		b.WriteString(fmt.Sprintf("%s %s\n\n", m.Spinner.View(), SystemStyle.Render(m.StatusLine)))
	} else {
		b.WriteString(InputPrompt + m.TextInput.View())
		b.WriteString(SystemStyle.Render("(Pressione Esc ou Ctrl+C para sair)"))
	}

	return b.String()
}

func InitialModel(q *swarm.Queen, p []swarm.Specialist) CLIModel {
	ti := textinput.New()
	ti.Placeholder = "Diga à Rainha o que deseja fazer..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 60

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return CLIModel{
		Queen:     q,
		Pipeline:  p,
		TextInput: ti,
		Spinner:   s,
		History:   []string{SystemStyle.Render("A Colmeia Jandaira despertou. As operárias aguardam as suas ordens.")},
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
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.IsWorking || m.TextInput.Value() == "" {
				return m, nil
			}
			goal := m.TextInput.Value()
			m.TextInput.SetValue("")
			m.History = append(m.History, fmt.Sprintf("👤 %s", goal))
			m.IsWorking = true
			m.StatusLine = "A Rainha está a analisar a tarefa..."
			cmds = append(cmds, m.runGoal(goal))
		}
	case ResultMsg:
		m.IsWorking = false
		if msg.err != nil {
			m.History = append(m.History, ErrorStyle.Render(fmt.Sprintf("❌ Erro: %v", msg.err)))
		} else {
			m.History = append(m.History, QueenStyle.Render("👑 Relatório da Rainha:\n")+msg.content)
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

package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/damiaoterto/jandaira/internal/config"
)

type WizardModel struct {
	config   *config.Config
	index    int
	input    textinput.Model
	styles   *Styles
	width    int
	done     bool
	err      error
	savePath string
}

func NewWizardModel(filepath string) WizardModel {
	styles := DefaultStyles()

	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	// width will be set dynamically via WindowSizeMsg
	ti.Width = 50
	ti.PromptStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Secondary).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Primary)

	m := WizardModel{
		config: &config.Config{
			Model:      "gpt-4o-mini",
			SwarmName:  "enxame-alfa",
			MaxNectar:  20000,
			Supervised: true,
			Isolated:   true,
		},
		input:    ti,
		styles:   styles,
		savePath: filepath,
	}

	m.updatePrompt()
	return m
}

func (m *WizardModel) updatePrompt() {
	switch m.index {
	case 0:
		m.input.Prompt = "1. Salvar configuração em "
		m.input.Placeholder = m.savePath
	case 1:
		m.input.Prompt = "2. Modelo OpenAI "
		m.input.Placeholder = m.config.Model
	case 2:
		m.input.Prompt = "3. Nome do Enxame "
		m.input.Placeholder = m.config.SwarmName
	case 3:
		m.input.Prompt = "4. Limite de Néctar (Tokens) "
		m.input.Placeholder = fmt.Sprintf("%d", m.config.MaxNectar)
	case 4:
		m.input.Prompt = "5. Modo Supervisionado? (S/n) "
		m.input.Placeholder = "s"
	case 5:
		m.input.Prompt = "6. Modo Isolado / Sandbox Wasm? (S/n) "
		m.input.Placeholder = "s"
	}
	m.input.SetValue("") // reset value
}

func (m WizardModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done || m.err != nil {
		return m, tea.Quit
	}

	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		// Maintain a reasonable width for the input text area
		if m.width > 20 {
			m.input.Width = m.width - 20
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			val := strings.TrimSpace(m.input.Value())

			// Process current answer
			switch m.index {
			case 0:
				if val != "" {
					m.savePath = val
				}
			case 1:
				if val != "" {
					m.config.Model = val
				}
			case 2:
				if val != "" {
					m.config.SwarmName = val
				}
			case 3:
				if val != "" {
					n, err := strconv.Atoi(val)
					if err == nil && n > 0 {
						m.config.MaxNectar = n
					}
				}
			case 4:
				m.config.Supervised = strings.ToLower(val) != "n"
			case 5:
				m.config.Isolated = strings.ToLower(val) != "n"
			}

			m.index++

			// Finished asking?
			if m.index > 5 {
				m.done = true
				err := config.Save(m.savePath, m.config)
				if err != nil {
					m.err = err
				}
				// Quit right after setting done to true so main.go can resume
				return m, tea.Quit
			}

			// Prepare next question
			m.updatePrompt()
		}
	}

	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m WizardModel) View() string {
	width := m.width
	if width == 0 {
		width = 80 // fallback before WindowSizeMsg
	}

	contentWidth := width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	var b strings.Builder

	// Banner
	bannerTitle := lipgloss.JoinVertical(
		lipgloss.Center,
		m.styles.Title.Render("🍯 Configuração Inicial 🍯"),
		m.styles.Subtitle.Render("Primeiro voo da Colmeia Jandaira"),
	)
	bannerBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(DefaultTheme.Secondary).
		Width(contentWidth).
		Align(lipgloss.Center).
		MarginBottom(1)

	b.WriteString(bannerBox.Render(bannerTitle))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(m.styles.ErrorBubble.Render("Erro ao salvar configuração: " + m.err.Error()))
		return m.styles.App.Render(b.String())
	}

	if m.done {
		b.WriteString(m.styles.Status.Render("✅ Configuração concluída!"))
		return m.styles.App.Render(b.String())
	}

	b.WriteString(m.styles.SystemMessage.Render("A colmeia precisa de algumas definições antes de acordar."))
	b.WriteString("\n\n")

	// Render the text input
	b.WriteString(m.input.View())
	b.WriteString("\n\n")
	b.WriteString(m.styles.Footer.Render("↵ confirmar • esc sair (deixe em branco para o padrão)"))

	return m.styles.App.Render(b.String())
}

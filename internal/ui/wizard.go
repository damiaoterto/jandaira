package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/damiaoterto/jandaira/internal/i18n"
	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/security"
	"github.com/damiaoterto/jandaira/internal/service"
)

// totalWizardSteps is the number of questions in the wizard (indices 0..totalWizardSteps-1).
// Steps: 0=language, 1=api_key, 2=model, 3=swarm_name, 4=max_nectar, 5=max_agents,
//
//	6=supervised, 7=isolated
const totalWizardSteps = 8

type WizardModel struct {
	config     *model.AppConfig
	apiKey     string
	index      int
	input      textinput.Model
	styles     *Styles
	width      int
	done       bool
	err        error
	cfgService service.ConfigService
}

func NewWizardModel(cfgService service.ConfigService) WizardModel {
	i18n.Init()
	styles := DefaultStyles()

	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50
	ti.PromptStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Secondary).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(DefaultTheme.Primary)

	m := WizardModel{
		config: &model.AppConfig{
			Language:   i18n.CurrentLang(),
			Model:      "gpt-4o-mini",
			SwarmName:  "enxame-alfa",
			MaxNectar:  20000,
			MaxAgents:  3,
			Supervised: true,
			Isolated:   true,
		},
		input:      ti,
		styles:     styles,
		cfgService: cfgService,
	}

	m.updatePrompt()
	return m
}

func (m *WizardModel) updatePrompt() {
	m.input.EchoMode = textinput.EchoNormal

	switch m.index {
	case 0:
		m.input.Prompt = i18n.T("wizard_prompt_lang")
		m.input.Placeholder = m.config.Language
	case 1:
		m.input.EchoMode = textinput.EchoPassword
		m.input.EchoCharacter = '*'
		m.input.Prompt = i18n.T("wizard_prompt_api_key")
		m.input.Placeholder = i18n.T("wizard_place_api_key")
	case 2:
		m.input.Prompt = i18n.T("wizard_prompt_model")
		m.input.Placeholder = m.config.Model
	case 3:
		m.input.Prompt = i18n.T("wizard_prompt_swarm")
		m.input.Placeholder = m.config.SwarmName
	case 4:
		m.input.Prompt = i18n.T("wizard_prompt_nectar")
		m.input.Placeholder = fmt.Sprintf("%d", m.config.MaxNectar)
	case 5:
		m.input.Prompt = i18n.T("wizard_prompt_max_agents")
		m.input.Placeholder = fmt.Sprintf("%d", m.config.MaxAgents)
	case 6:
		m.input.Prompt = i18n.T("wizard_prompt_supervised")
		m.input.Placeholder = "s"
	case 7:
		m.input.Prompt = i18n.T("wizard_prompt_isolated")
		m.input.Placeholder = "s"
	}
	m.input.SetValue("")
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

			switch m.index {
			case 0:
				if val != "" {
					m.config.Language = val
					i18n.SetLanguage(val)
				}
			case 1:
				if val != "" {
					m.apiKey = val
				}
			case 2:
				if val != "" {
					m.config.Model = val
				}
			case 3:
				if val != "" {
					m.config.SwarmName = val
				}
			case 4:
				if val != "" {
					n, err := strconv.Atoi(val)
					if err == nil && n > 0 {
						m.config.MaxNectar = n
					}
				}
			case 5:
				if val != "" {
					n, err := strconv.Atoi(val)
					if err == nil && n > 0 {
						m.config.MaxAgents = n
					}
				}
			case 6:
				if val != "" {
					m.config.Supervised = strings.ToLower(val) != "n"
				}
			case 7:
				if val != "" {
					m.config.Isolated = strings.ToLower(val) != "n"
				}
			}

			m.index++

			if m.index >= totalWizardSteps {
				if m.apiKey != "" {
					repoDir := security.GetDefaultVaultDir()
					v, vErr := security.InitVault(repoDir)
					if vErr != nil {
						m.err = fmt.Errorf("erro ao inicializar cofre de segredos: %w", vErr)
						return m, tea.Quit
					}
					if saveErr := v.SaveSecret("OPENAI_API_KEY", m.apiKey); saveErr != nil {
						m.err = fmt.Errorf("erro ao salvar chave API no cofre: %w", saveErr)
						return m, tea.Quit
					}
				}

				if saveErr := m.cfgService.Save(m.config); saveErr != nil {
					m.err = saveErr
					return m, tea.Quit
				}

				m.done = true
				return m, tea.Quit
			}

			m.updatePrompt()
		}
	}

	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m WizardModel) View() string {
	width := m.width
	if width == 0 {
		width = 80
	}

	contentWidth := width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	var b strings.Builder

	bannerTitle := lipgloss.JoinVertical(
		lipgloss.Center,
		m.styles.Title.Render(i18n.T("wizard_title")),
		m.styles.Subtitle.Render(i18n.T("wizard_subtitle")),
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
		b.WriteString(m.styles.ErrorBubble.Render(i18n.T("wizard_err_save") + m.err.Error()))
		return m.styles.App.Render(b.String())
	}

	if m.done {
		b.WriteString(m.styles.Status.Render(i18n.T("wizard_success")))
		return m.styles.App.Render(b.String())
	}

	b.WriteString(m.styles.SystemMessage.Render(i18n.T("wizard_system_msg")))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")
	b.WriteString(m.styles.Footer.Render(i18n.T("wizard_footer")))

	return m.styles.App.Render(b.String())
}

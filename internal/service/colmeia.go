package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/repository"
	"github.com/damiaoterto/jandaira/internal/swarm"
)

var (
	ErrColmeiaNotFound       = errors.New("colmeia não encontrada")
	ErrAgenteColmeiaNotFound = errors.New("agente da colmeia não encontrado")
)

// ColmeiaService define a lógica de negócio para gerenciamento de Colmeias.
type ColmeiaService interface {
	// Colmeia lifecycle
	CreateColmeia(name, description string, queenManaged bool) (*model.Colmeia, error)
	GetColmeia(id string) (*model.Colmeia, error)
	ListColmeias() ([]model.Colmeia, error)
	UpdateColmeia(id, name, description string, queenManaged bool) (*model.Colmeia, error)
	DeleteColmeia(id string) error

	// Gerenciamento de agentes
	AddAgente(colmeiaID, name, systemPrompt string, allowedTools []string) (*model.AgenteColmeia, error)
	UpdateAgente(agenteID uint, name, systemPrompt string, allowedTools []string) (*model.AgenteColmeia, error)
	RemoveAgente(agenteID uint) error
	ListAgentes(colmeiaID string) ([]model.AgenteColmeia, error)
	GetAgente(agenteID uint) (*model.AgenteColmeia, error)

	// Histórico de despachos
	CreateHistorico(colmeiaID, goal string) (*model.HistoricoDespacho, error)
	CompleteHistorico(id, result string) error
	FailHistorico(id string) error
	ListHistorico(colmeiaID string) ([]model.HistoricoDespacho, error)

	// Helpers para despacho
	BuildSpecialists(colmeia *model.Colmeia) []swarm.Specialist
	BuildGoalWithHistory(colmeia *model.Colmeia, goal string) (string, error)
	// BuildSkillsContext gera um bloco de texto descrevendo as skills da colmeia
	// para injetar no prompt da rainha durante o AssembleSwarm.
	BuildSkillsContext(colmeia *model.Colmeia) string
}

type colmeiaService struct {
	colmeias  repository.ColmeiaRepository
	agentes   repository.AgenteColmeiaRepository
	historico repository.HistoricoDespachoRepository
}

// NewColmeiaService cria um novo ColmeiaService.
func NewColmeiaService(
	colmeias repository.ColmeiaRepository,
	agentes repository.AgenteColmeiaRepository,
	historico repository.HistoricoDespachoRepository,
) ColmeiaService {
	return &colmeiaService{
		colmeias:  colmeias,
		agentes:   agentes,
		historico: historico,
	}
}

func (s *colmeiaService) CreateColmeia(name, description string, queenManaged bool) (*model.Colmeia, error) {
	c := &model.Colmeia{
		Name:         name,
		Description:  description,
		QueenManaged: queenManaged,
	}
	if err := s.colmeias.Create(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *colmeiaService) GetColmeia(id string) (*model.Colmeia, error) {
	c, err := s.colmeias.FindByIDWithAgentes(id)
	if err != nil {
		if errors.Is(err, repository.ErrColmeiaNotFound) {
			return nil, ErrColmeiaNotFound
		}
		return nil, err
	}
	return c, nil
}

func (s *colmeiaService) ListColmeias() ([]model.Colmeia, error) {
	return s.colmeias.FindAll()
}

func (s *colmeiaService) UpdateColmeia(id, name, description string, queenManaged bool) (*model.Colmeia, error) {
	c, err := s.colmeias.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrColmeiaNotFound) {
			return nil, ErrColmeiaNotFound
		}
		return nil, err
	}
	c.Name = name
	c.Description = description
	c.QueenManaged = queenManaged
	if err := s.colmeias.Update(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *colmeiaService) DeleteColmeia(id string) error {
	if _, err := s.colmeias.FindByID(id); err != nil {
		if errors.Is(err, repository.ErrColmeiaNotFound) {
			return ErrColmeiaNotFound
		}
		return err
	}
	return s.colmeias.Delete(id)
}

func (s *colmeiaService) AddAgente(colmeiaID, name, systemPrompt string, allowedTools []string) (*model.AgenteColmeia, error) {
	if _, err := s.colmeias.FindByID(colmeiaID); err != nil {
		if errors.Is(err, repository.ErrColmeiaNotFound) {
			return nil, ErrColmeiaNotFound
		}
		return nil, err
	}
	a := &model.AgenteColmeia{
		ColmeiaID:    colmeiaID,
		Name:         name,
		SystemPrompt: systemPrompt,
	}
	if err := a.SetAllowedTools(allowedTools); err != nil {
		return nil, fmt.Errorf("erro ao serializar ferramentas: %w", err)
	}
	if err := s.agentes.Create(a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *colmeiaService) UpdateAgente(agenteID uint, name, systemPrompt string, allowedTools []string) (*model.AgenteColmeia, error) {
	a, err := s.agentes.FindByID(agenteID)
	if err != nil {
		if errors.Is(err, repository.ErrAgenteColmeiaNotFound) {
			return nil, ErrAgenteColmeiaNotFound
		}
		return nil, err
	}
	a.Name = name
	a.SystemPrompt = systemPrompt
	if err := a.SetAllowedTools(allowedTools); err != nil {
		return nil, fmt.Errorf("erro ao serializar ferramentas: %w", err)
	}
	if err := s.agentes.Update(a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *colmeiaService) RemoveAgente(agenteID uint) error {
	if _, err := s.agentes.FindByID(agenteID); err != nil {
		if errors.Is(err, repository.ErrAgenteColmeiaNotFound) {
			return ErrAgenteColmeiaNotFound
		}
		return err
	}
	return s.agentes.Delete(agenteID)
}

func (s *colmeiaService) ListAgentes(colmeiaID string) ([]model.AgenteColmeia, error) {
	if _, err := s.colmeias.FindByID(colmeiaID); err != nil {
		if errors.Is(err, repository.ErrColmeiaNotFound) {
			return nil, ErrColmeiaNotFound
		}
		return nil, err
	}
	return s.agentes.FindByColmeiaID(colmeiaID)
}

func (s *colmeiaService) GetAgente(agenteID uint) (*model.AgenteColmeia, error) {
	a, err := s.agentes.FindByID(agenteID)
	if err != nil {
		if errors.Is(err, repository.ErrAgenteColmeiaNotFound) {
			return nil, ErrAgenteColmeiaNotFound
		}
		return nil, err
	}
	return a, nil
}

func (s *colmeiaService) CreateHistorico(colmeiaID, goal string) (*model.HistoricoDespacho, error) {
	h := &model.HistoricoDespacho{
		ColmeiaID: colmeiaID,
		Goal:      goal,
		Status:    model.HistoricoStatusActive,
	}
	if err := s.historico.Create(h); err != nil {
		return nil, err
	}
	return h, nil
}

func (s *colmeiaService) CompleteHistorico(id, result string) error {
	h, err := s.historico.FindByID(id)
	if err != nil {
		return err
	}
	h.Status = model.HistoricoStatusCompleted
	h.Result = result
	return s.historico.Update(h)
}

func (s *colmeiaService) FailHistorico(id string) error {
	h, err := s.historico.FindByID(id)
	if err != nil {
		return err
	}
	h.Status = model.HistoricoStatusFailed
	return s.historico.Update(h)
}

func (s *colmeiaService) ListHistorico(colmeiaID string) ([]model.HistoricoDespacho, error) {
	if _, err := s.colmeias.FindByID(colmeiaID); err != nil {
		if errors.Is(err, repository.ErrColmeiaNotFound) {
			return nil, ErrColmeiaNotFound
		}
		return nil, err
	}
	return s.historico.FindByColmeiaID(colmeiaID)
}

// BuildSpecialists converte os AgenteColmeia da colmeia em swarm.Specialist.
// Usado quando QueenManaged=false (agentes definidos pelo usuário).
// Skills do agente são mescladas: instruções adicionadas ao SystemPrompt e
// ferramentas permitidas unidas sem duplicatas.
func (s *colmeiaService) BuildSpecialists(colmeia *model.Colmeia) []swarm.Specialist {
	specialists := make([]swarm.Specialist, 0, len(colmeia.Agentes))
	for _, a := range colmeia.Agentes {
		systemPrompt := a.SystemPrompt
		allowedTools := a.GetAllowedTools()

		for _, skill := range a.Skills {
			if skill.Instructions != "" {
				systemPrompt += fmt.Sprintf("\n\n[SKILL: %s]\n%s", skill.Name, skill.Instructions)
			}
			for _, t := range skill.GetAllowedTools() {
				allowedTools = appendIfMissing(allowedTools, t)
			}
		}

		specialists = append(specialists, swarm.Specialist{
			Name:         a.Name,
			SystemPrompt: systemPrompt,
			AllowedTools: allowedTools,
		})
	}
	return specialists
}

// BuildSkillsContext gera um bloco de texto com as skills da colmeia para
// injetar no prompt da rainha durante o AssembleSwarm (queen_managed=true).
func (s *colmeiaService) BuildSkillsContext(colmeia *model.Colmeia) string {
	if len(colmeia.Skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("SKILLS DISPONÍVEIS PARA ESTA COLMEIA (atribua as relevantes aos especialistas):\n")
	for _, skill := range colmeia.Skills {
		sb.WriteString(fmt.Sprintf("- \"%s\": %s", skill.Name, skill.Description))
		if skill.Instructions != "" {
			instr := truncate(skill.Instructions, 300)
			sb.WriteString(fmt.Sprintf(". Instruções: %s", instr))
		}
		tools := skill.GetAllowedTools()
		if len(tools) > 0 {
			sb.WriteString(fmt.Sprintf(". Ferramentas: [%s]", strings.Join(tools, ", ")))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// appendIfMissing adiciona t à slice somente se ainda não estiver presente.
func appendIfMissing(slice []string, t string) []string {
	for _, v := range slice {
		if v == t {
			return slice
		}
	}
	return append(slice, t)
}

// BuildGoalWithHistory enriquece o objetivo com o histórico recente da colmeia,
// permitindo que os agentes tenham contexto de conversas anteriores.
func (s *colmeiaService) BuildGoalWithHistory(colmeia *model.Colmeia, goal string) (string, error) {
	recent, err := s.historico.FindRecentCompleted(colmeia.ID, 3)
	if err != nil || len(recent) == 0 {
		return goal, nil
	}

	var sb strings.Builder
	sb.WriteString("HISTÓRICO DE CONVERSAS ANTERIORES DESTA COLMEIA (use como contexto):\n")
	for _, h := range recent {
		// Extract only the final agent report — skip the injected history header
		// from previous dispatches to prevent recursive context nesting.
		summary := extractLastReport(h.Result)
		sb.WriteString(fmt.Sprintf("\n--- Mensagem anterior ---\nObjetivo: %s\nResultado: %s\n",
			h.Goal, truncate(summary, 300)))
	}
	sb.WriteString("\n--- Nova mensagem do usuário ---\n")
	sb.WriteString(goal)
	return sb.String(), nil
}

// extractLastReport returns the content of the last "--- Report from X ---"
// block in the contextAccumulator. Falls back to the raw string if no block found.
func extractLastReport(raw string) string {
	const marker = "--- Report from "
	idx := strings.LastIndex(raw, marker)
	if idx < 0 {
		return strings.TrimSpace(raw)
	}
	// Skip the marker line ("--- Report from X ---\n")
	after := raw[idx:]
	newline := strings.Index(after, "\n")
	if newline < 0 {
		return strings.TrimSpace(raw)
	}
	return strings.TrimSpace(after[newline+1:])
}

// truncate limita uma string ao tamanho máximo informado.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

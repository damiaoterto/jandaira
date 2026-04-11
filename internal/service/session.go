package service

import (
	"errors"

	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/repository"
)

// ErrSessionNotFound is returned when no session with the given ID exists.
var ErrSessionNotFound = errors.New("sessão não encontrada")

// SessionService defines the business logic for sessions and their agents.
type SessionService interface {
	// Session lifecycle
	Create(name, goal string) (*model.Session, error)
	GetSession(id string) (*model.Session, error)
	ListSessions() ([]model.Session, error)
	DeleteSession(id string) error
	CompleteSession(id, result string) error
	FailSession(id string) error

	// Agent management
	AddAgent(sessionID, name, role string) (*model.Agent, error)
	ListAgents(sessionID string) ([]model.Agent, error)
	UpdateAgentStatusByName(sessionID, name, status string) error
}

type sessionService struct {
	sessions repository.SessionRepository
	agents   repository.AgentRepository
}

// NewSessionService creates a new SessionService backed by the given repositories.
func NewSessionService(
	sessions repository.SessionRepository,
	agents repository.AgentRepository,
) SessionService {
	return &sessionService{sessions: sessions, agents: agents}
}

// Create starts a new active session.
func (s *sessionService) Create(name, goal string) (*model.Session, error) {
	session := &model.Session{
		Name:   name,
		Goal:   goal,
		Status: model.SessionStatusActive,
	}
	if err := s.sessions.Create(session); err != nil {
		return nil, err
	}
	return session, nil
}

// GetSession returns the session with all its agents eagerly loaded.
func (s *sessionService) GetSession(id string) (*model.Session, error) {
	session, err := s.sessions.FindByIDWithAgents(id)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return session, nil
}

// ListSessions returns all sessions ordered by creation date (newest first).
// Agents are not included to keep the payload lightweight.
func (s *sessionService) ListSessions() ([]model.Session, error) {
	return s.sessions.FindAll()
}

// DeleteSession removes the session and all its agents.
// Agents are deleted explicitly before the session as a safety net, since
// SQLite foreign-key cascade requires PRAGMA foreign_keys = ON.
func (s *sessionService) DeleteSession(id string) error {
	if _, err := s.sessions.FindByID(id); err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return ErrSessionNotFound
		}
		return err
	}
	// Delete agents explicitly (safe even if FK cascade fires too).
	if err := s.agents.DeleteBySessionID(id); err != nil {
		return err
	}
	return s.sessions.Delete(id)
}

// CompleteSession marks the session as completed, stores the result, and sets
// all its agents to "done".
func (s *sessionService) CompleteSession(id, result string) error {
	session, err := s.sessions.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return ErrSessionNotFound
		}
		return err
	}
	session.Status = model.SessionStatusCompleted
	session.Result = result
	if err := s.sessions.Update(session); err != nil {
		return err
	}
	return s.agents.UpdateAllInSession(id, model.AgentStatusDone)
}

// FailSession marks the session and all its agents as failed.
func (s *sessionService) FailSession(id string) error {
	session, err := s.sessions.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return ErrSessionNotFound
		}
		return err
	}
	session.Status = model.SessionStatusFailed
	if err := s.sessions.Update(session); err != nil {
		return err
	}
	return s.agents.UpdateAllInSession(id, model.AgentStatusFailed)
}

// AddAgent registers a new agent under the given session.
func (s *sessionService) AddAgent(sessionID, name, role string) (*model.Agent, error) {
	if _, err := s.sessions.FindByID(sessionID); err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	agent := &model.Agent{
		SessionID: sessionID,
		Name:      name,
		Role:      role,
		Status:    model.AgentStatusIdle,
	}
	if err := s.agents.Create(agent); err != nil {
		return nil, err
	}
	return agent, nil
}

// ListAgents returns all agents belonging to the given session.
func (s *sessionService) ListAgents(sessionID string) ([]model.Agent, error) {
	if _, err := s.sessions.FindByID(sessionID); err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return s.agents.FindBySessionID(sessionID)
}

// UpdateAgentStatusByName updates the status of an agent identified by its
// name within the given session. Used by AgentChangeFunc callbacks.
func (s *sessionService) UpdateAgentStatusByName(sessionID, name, status string) error {
	return s.agents.UpdateStatusByName(sessionID, name, status)
}

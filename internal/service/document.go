package service

import (
	"errors"

	"github.com/damiaoterto/jandaira/internal/model"
	"github.com/damiaoterto/jandaira/internal/repository"
)

var ErrDocumentNotFound = errors.New("document not found")

type DocumentService interface {
	Create(filename, workspacePath, collection, scopeKey, scopeVal string, chunks int) (*model.Document, error)
	GetDocument(id string) (*model.Document, error)
	ListByScope(scopeKey, scopeVal string) ([]model.Document, error)
	Delete(id string) error
}

type documentService struct {
	docs repository.DocumentRepository
}

func NewDocumentService(docs repository.DocumentRepository) DocumentService {
	return &documentService{docs: docs}
}

func (s *documentService) Create(filename, workspacePath, collection, scopeKey, scopeVal string, chunks int) (*model.Document, error) {
	d := &model.Document{
		Filename:      filename,
		WorkspacePath: workspacePath,
		Collection:    collection,
		ScopeKey:      scopeKey,
		ScopeVal:      scopeVal,
		Chunks:        chunks,
	}
	if err := s.docs.Create(d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *documentService) GetDocument(id string) (*model.Document, error) {
	d, err := s.docs.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrDocumentNotFound) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}
	return d, nil
}

func (s *documentService) ListByScope(scopeKey, scopeVal string) ([]model.Document, error) {
	return s.docs.FindByScopeVal(scopeKey, scopeVal)
}

func (s *documentService) Delete(id string) error {
	if _, err := s.docs.FindByID(id); err != nil {
		if errors.Is(err, repository.ErrDocumentNotFound) {
			return ErrDocumentNotFound
		}
		return err
	}
	return s.docs.Delete(id)
}

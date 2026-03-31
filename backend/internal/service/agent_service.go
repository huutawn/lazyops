package service

import (
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/internal/repository"
)

type AgentService struct {
	agents *repository.AgentRepository
}

func NewAgentService(agents *repository.AgentRepository) *AgentService {
	return &AgentService{agents: agents}
}

func (s *AgentService) List() ([]AgentRecord, error) {
	agents, err := s.agents.List()
	if err != nil {
		return nil, err
	}

	items := make([]AgentRecord, 0, len(agents))
	for _, agent := range agents {
		items = append(items, ToAgentRecord(agent))
	}
	return items, nil
}

func (s *AgentService) Create(cmd CreateAgentCommand) (*AgentRecord, error) {
	if strings.TrimSpace(cmd.AgentID) == "" || strings.TrimSpace(cmd.Name) == "" {
		return nil, ErrInvalidInput
	}

	agent := &models.Agent{
		AgentID: strings.TrimSpace(cmd.AgentID),
		Name:    strings.TrimSpace(cmd.Name),
		Status:  normalizeAgentStatus(cmd.Status),
	}
	if err := s.agents.Create(agent); err != nil {
		return nil, err
	}

	record := ToAgentRecord(*agent)
	return &record, nil
}

func (s *AgentService) UpdateStatus(cmd UpdateAgentStatusCommand) (*AgentRecord, error) {
	if strings.TrimSpace(cmd.AgentID) == "" {
		return nil, ErrInvalidInput
	}

	agent, err := s.agents.UpsertStatus(
		strings.TrimSpace(cmd.AgentID),
		strings.TrimSpace(cmd.Name),
		normalizeAgentStatus(cmd.Status),
	)
	if err != nil {
		return nil, err
	}

	record := ToAgentRecord(*agent)
	return &record, nil
}

func (s *AgentService) BuildRealtimeEvent(agent AgentRecord, source string) AgentRealtimeEvent {
	return AgentRealtimeEvent{
		Type:    "agent.status.changed",
		Payload: agent,
		Meta: RealtimeMeta{
			Source: source,
			At:     time.Now().UTC(),
		},
	}
}

func ToAgentRecord(agent models.Agent) AgentRecord {
	return AgentRecord{
		ID:         agent.ID,
		AgentID:    agent.AgentID,
		Name:       agent.Name,
		Status:     agent.Status,
		LastSeenAt: agent.LastSeenAt,
		UpdatedAt:  agent.UpdatedAt,
	}
}

func normalizeAgentStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online":
		return "online"
	case "busy":
		return "busy"
	case "error":
		return "error"
	default:
		return "offline"
	}
}

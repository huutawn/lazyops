package service

import (
	"errors"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeAgentStore struct {
	agents []*models.Agent
}

func (f *fakeAgentStore) Create(agent *models.Agent) error {
	f.agents = append(f.agents, agent)
	return nil
}

func (f *fakeAgentStore) ListByUser(userID string) ([]models.Agent, error) {
	items := make([]models.Agent, 0)
	for _, agent := range f.agents {
		if agent.UserID == userID {
			items = append(items, *agent)
		}
	}
	return items, nil
}

func (f *fakeAgentStore) GetByAgentIDForUser(userID, agentID string) (*models.Agent, error) {
	for _, agent := range f.agents {
		if agent.UserID == userID && agent.AgentID == agentID {
			return agent, nil
		}
	}
	return nil, nil
}

func (f *fakeAgentStore) UpdateStatusForUser(userID, agentID, name, status string, at time.Time) (*models.Agent, error) {
	for _, agent := range f.agents {
		if agent.UserID == userID && agent.AgentID == agentID {
			agent.Name = name
			agent.Status = status
			agent.LastSeenAt = &at
			agent.UpdatedAt = at
			return agent, nil
		}
	}
	return nil, nil
}

func TestAgentServiceListIsScopedByOwner(t *testing.T) {
	store := &fakeAgentStore{
		agents: []*models.Agent{
			{ID: "agt_1", UserID: "usr_a", AgentID: "agent-a", Name: "Agent A", Status: "online"},
			{ID: "agt_2", UserID: "usr_b", AgentID: "agent-b", Name: "Agent B", Status: "offline"},
		},
	}
	service := NewAgentService(store)

	items, err := service.List("usr_a")
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 owned agent, got %d", len(items))
	}
	if items[0].UserID != "usr_a" {
		t.Fatalf("expected only usr_a agent, got owner %q", items[0].UserID)
	}
}

func TestAgentServiceUpdateStatusRejectsCrossUserResource(t *testing.T) {
	store := &fakeAgentStore{
		agents: []*models.Agent{
			{ID: "agt_1", UserID: "usr_a", AgentID: "agent-a", Name: "Agent A", Status: "online"},
		},
	}
	service := NewAgentService(store)

	_, err := service.UpdateStatus(UpdateAgentStatusCommand{
		UserID:  "usr_b",
		AgentID: "agent-a",
		Name:    "Other User Attempt",
		Status:  "busy",
	})
	if !errors.Is(err, ErrAgentNotFound) {
		t.Fatalf("expected ErrAgentNotFound for cross-user update, got %v", err)
	}
}

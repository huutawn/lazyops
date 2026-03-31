package request

type CreateAgentRequest struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
}

type UpdateAgentStatusRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Source string `json:"source"`
}

type AgentStatusWSMessage struct {
	Type    string `json:"type"`
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
}

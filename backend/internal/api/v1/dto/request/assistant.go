package request

type CreateAssistantSessionRequest struct {
	ProjectID string `json:"project_id"`
	Title     string `json:"title"`
}

type PostAssistantMessageRequest struct {
	ProjectID string `json:"project_id"`
	Content   string `json:"content" binding:"required"`
}

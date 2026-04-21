package request

type AllocateProjectDomainRequest struct {
	Regenerate bool `json:"regenerate,omitempty"`
}

type RenameProjectDomainRequest struct {
	Label string `json:"label" binding:"required"`
}

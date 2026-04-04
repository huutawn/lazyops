package request

type CreateTunnelSessionRequest struct {
	ProjectID string `json:"project_id"`
	TargetRef string `json:"target_ref"`
	LocalPort int    `json:"local_port"`
	Remote    string `json:"remote"`
	Timeout   string `json:"timeout"`
}

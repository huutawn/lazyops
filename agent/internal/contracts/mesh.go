package contracts

type EnsureMeshPeerPayload struct {
	ProjectID    string       `json:"project_id"`
	BindingID    string       `json:"binding_id"`
	RevisionID   string       `json:"revision_id,omitempty"`
	RuntimeMode  RuntimeMode  `json:"runtime_mode"`
	Provider     MeshProvider `json:"provider"`
	PeerRef      string       `json:"peer_ref,omitempty"`
	TargetID     string       `json:"target_id,omitempty"`
	TargetKind   TargetKind   `json:"target_kind,omitempty"`
	DesiredState string       `json:"desired_state,omitempty"`
}

type SyncOverlayRoutesPayload struct {
	ProjectID   string       `json:"project_id"`
	BindingID   string       `json:"binding_id"`
	RevisionID  string       `json:"revision_id,omitempty"`
	RuntimeMode RuntimeMode  `json:"runtime_mode"`
	Provider    MeshProvider `json:"provider,omitempty"`
}

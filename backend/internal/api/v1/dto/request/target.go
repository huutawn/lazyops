package request

type CreateMeshNetworkRequest struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	CIDR     string `json:"cidr"`
}

type CreateClusterRequest struct {
	Name                string `json:"name"`
	Provider            string `json:"provider"`
	KubeconfigSecretRef string `json:"kubeconfig_secret_ref"`
}

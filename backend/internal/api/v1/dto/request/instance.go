package request

type CreateInstanceRequest struct {
	Name      string            `json:"name"`
	PublicIP  string            `json:"public_ip"`
	PrivateIP string            `json:"private_ip"`
	Labels    map[string]string `json:"labels"`
}

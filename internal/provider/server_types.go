package provider

// createServerPayload matches the subset of dto.CreateServerRequest fields we
// actually allow Terraform to manage. We deliberately omit "status" etc.
type createServerPayload struct {
	Hostname         string `json:"hostname,omitempty"`
	Role             string `json:"role,omitempty"`
	PrivateIPAddress string `json:"private_ip_address,omitempty"`
	PublicIPAddress  string `json:"public_ip_address,omitempty"`
	SSHKeyID         string `json:"ssh_key_id,omitempty"`
	SSHUser          string `json:"ssh_user,omitempty"`
}

// For updates, the API has a separate dto.UpdateServerRequest, but its fields
// are effectively the same subset we care about, so we reuse the same payload.
type updateServerPayload = createServerPayload

// server represents dto.ServerResponse (subset of fields we care about).
type server struct {
	ID               string `json:"id"`
	Hostname         string `json:"hostname"`
	Role             string `json:"role"`
	PrivateIPAddress string `json:"private_ip_address"`
	PublicIPAddress  string `json:"public_ip_address"`
	SSHKeyID         string `json:"ssh_key_id"`
	SSHUser          string `json:"ssh_user"`
	OrganizationID   string `json:"organization_id"`
	Status           string `json:"status"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

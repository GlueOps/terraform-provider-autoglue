package provider

type createLoadBalancerPayload struct {
	Name             string `json:"name,omitempty"`
	Kind             string `json:"kind,omitempty"`
	PublicIPAddress  string `json:"public_ip_address,omitempty"`
	PrivateIPAddress string `json:"private_ip_address,omitempty"`
}

type updateLoadBalancerPayload struct {
	Name             *string `json:"name,omitempty"`
	Kind             *string `json:"kind,omitempty"`
	PublicIPAddress  *string `json:"public_ip_address,omitempty"`
	PrivateIPAddress *string `json:"private_ip_address,omitempty"`
}

type loadBalancer struct {
	ID               string `json:"id"`
	OrganizationID   string `json:"organization_id"`
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	PublicIPAddress  string `json:"public_ip_address"`
	PrivateIPAddress string `json:"private_ip_address"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

package provider

type createDomainPayload struct {
	DomainName   string `json:"domain_name"`
	CredentialID string `json:"credential_id"`
	ZoneID       string `json:"zone_id,omitempty"`
}

type updateDomainPayload struct {
	CredentialID *string `json:"credential_id,omitempty"`
	ZoneID       *string `json:"zone_id,omitempty"`
	Status       *string `json:"status,omitempty"`
	DomainName   *string `json:"domain_name,omitempty"`
}

type domain struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	DomainName     string `json:"domain_name"`
	ZoneID         string `json:"zone_id"`
	Status         string `json:"status"`
	LastError      string `json:"last_error"`
	CredentialID   string `json:"credential_id"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

package provider

// createCredentialPayload matches dto.CreateCredentialRequest.
type createCredentialPayload struct {
	Name               string            `json:"name,omitempty"`
	CredentialProvider string            `json:"credential_provider"`
	Kind               string            `json:"kind"`
	SchemaVersion      int32             `json:"schema_version"`
	Scope              map[string]string `json:"scope,omitempty"`
	ScopeKind          string            `json:"scope_kind,omitempty"`
	ScopeVersion       int32             `json:"scope_version,omitempty"`
	Secret             map[string]string `json:"secret"`
	AccountID          string            `json:"account_id,omitempty"`
	Region             string            `json:"region,omitempty"`
}

// updateCredentialPayload matches dto.UpdateCredentialRequest.
type updateCredentialPayload struct {
	Name         string            `json:"name,omitempty"`
	Scope        map[string]string `json:"scope,omitempty"`
	ScopeKind    string            `json:"scope_kind,omitempty"`
	ScopeVersion int32             `json:"scope_version,omitempty"`
	Secret       map[string]string `json:"secret,omitempty"`
	AccountID    string            `json:"account_id,omitempty"`
	Region       string            `json:"region,omitempty"`
}

// credential represents dto.CredentialOut.
type credential struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	CredentialProvider string            `json:"credential_provider"`
	Kind               string            `json:"kind"`
	SchemaVersion      int32             `json:"schema_version"`
	Scope              map[string]string `json:"scope"`
	ScopeKind          string            `json:"scope_kind"`
	ScopeVersion       int32             `json:"scope_version"`
	AccountID          string            `json:"account_id"`
	Region             string            `json:"region"`
	CreatedAt          string            `json:"created_at"`
	UpdatedAt          string            `json:"updated_at"`
}

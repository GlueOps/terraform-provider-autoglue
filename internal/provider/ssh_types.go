package provider

// createSSHKeyPayload matches dto.CreateSSHRequest.
type createSSHKeyPayload struct {
	Bits    *int   `json:"bits,omitempty"`
	Comment string `json:"comment,omitempty"`
	Name    string `json:"name,omitempty"`
	Type    string `json:"type,omitempty"` // "rsa" (default) or "ed25519"
}

// sshKey matches dto.SshResponse (no private key, safer for TF).
type sshKey struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Fingerprint    string `json:"fingerprint"`
	PublicKey      string `json:"public_key"`
	OrganizationID string `json:"organization_id"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

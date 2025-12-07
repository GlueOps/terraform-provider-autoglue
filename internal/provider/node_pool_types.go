package provider

// createNodePoolPayload matches dto.CreateNodePoolRequest.
type createNodePoolPayload struct {
	Name string `json:"name"`
	Role string `json:"role"` // "master" or "worker"
}

// updateNodePoolPayload matches dto.UpdateNodePoolRequest.
type updateNodePoolPayload struct {
	Name *string `json:"name,omitempty"`
	Role *string `json:"role,omitempty"` // "master" or "worker"
}

// nodePool matches the subset of dto.NodePoolResponse we care about in Terraform.
type nodePool struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`

	Name string `json:"name"`
	Role string `json:"role"` // "master" or "worker"
}

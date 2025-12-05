package provider

// createNodePoolPayload matches dto.CreateNodePoolRequest.
type createNodePoolPayload struct {
	Name           string            `json:"name"`
	ApiserverURL   string            `json:"apiserver_url"`
	KubeletVersion string            `json:"kubelet_version"`
	KubeletOptions map[string]string `json:"kubelet_options,omitempty"`
	Role           string            `json:"role"`
}

// updateNodePoolPayload matches dto.UpdateNodePoolRequest.
type updateNodePoolPayload struct {
	Name           *string           `json:"name,omitempty"`
	ApiserverURL   *string           `json:"apiserver_url,omitempty"`
	KubeletVersion *string           `json:"kubelet_version,omitempty"`
	KubeletOptions map[string]string `json:"kubelet_options,omitempty"`
	Role           *string           `json:"role,omitempty"`
}

// nodePool represents the subset of dto.NodePoolResponse we care about.
type nodePool struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	ApiserverURL   string            `json:"apiserver_url"`
	KubeletVersion string            `json:"kubelet_version"`
	KubeletOptions map[string]string `json:"kubelet_options,omitempty"`
	Role           string            `json:"role"`

	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	OrganizationID string `json:"organization_id"`
}

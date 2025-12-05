package provider

// createLabelPayload matches dto.CreateLabelRequest / dto.UpdateLabelRequest.
type createLabelPayload struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// label represents dto.LabelResponse.
type label struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Value          string `json:"value"`
	OrganizationID string `json:"organization_id"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

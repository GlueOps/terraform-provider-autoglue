package provider

// createAnnotationPayload matches dto.CreateAnnotationRequest / dto.UpdateAnnotationRequest.
type createAnnotationPayload struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// annotation represents dto.AnnotationResponse.
type annotation struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Value          string `json:"value"`
	OrganizationID string `json:"organization_id"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

package provider

// createTaintPayload matches dto.CreateTaintRequest.
type createTaintPayload struct {
	Key   string  `json:"key,omitempty"`
	Value *string `json:"value,omitempty"`
	// Kubernetes-style taint effect, for example "NoSchedule", "PreferNoSchedule", "NoExecute".
	Effect string `json:"effect,omitempty"`
}

// updateTaintPayload matches dto.UpdateTaintRequest (all fields optional).
type updateTaintPayload struct {
	Key    *string `json:"key,omitempty"`
	Value  *string `json:"value,omitempty"`
	Effect *string `json:"effect,omitempty"`
}

// taint represents dto.TaintResponse (subset of fields we care about in Terraform).
type taint struct {
	ID        string  `json:"id"`
	Key       string  `json:"key"`
	Value     *string `json:"value,omitempty"`
	Effect    string  `json:"effect"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

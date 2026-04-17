package provider

// createClusterMetadataPayload matches dto.CreateClusterMetadataRequest.
type createClusterMetadataPayload struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// updateClusterMetadataPayload matches dto.UpdateClusterMetadataRequest.
type updateClusterMetadataPayload struct {
	Key   *string `json:"key,omitempty"`
	Value *string `json:"value,omitempty"`
}

// clusterMetadata represents dto.ClusterMetadataResponse.
type clusterMetadata struct {
	ID             string `json:"id"`
	ClusterID      string `json:"cluster_id"`
	Key            string `json:"key"`
	Value          string `json:"value"`
	OrganizationID string `json:"organization_id"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

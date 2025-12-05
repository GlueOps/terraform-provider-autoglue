package provider

// createClusterPayload matches dto.CreateClusterRequest / dto.UpdateClusterRequest.
type createClusterPayload struct {
	Name            string `json:"name"`
	ClusterProvider string `json:"cluster_provider"`
	Region          string `json:"region"`
}

// cluster represents dto.ClusterResponse (subset of fields we care about in Terraform).
type cluster struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ClusterProvider string `json:"cluster_provider"`
	Region          string `json:"region"`
	Status          string `json:"status"`
}

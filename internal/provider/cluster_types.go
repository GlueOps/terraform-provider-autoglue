package provider

// createClusterPayload matches dto.CreateClusterRequest / dto.UpdateClusterRequest.
type createClusterPayload struct {
	Name            string `json:"name"`
	ClusterProvider string `json:"cluster_provider"`
	Region          string `json:"region"`
	DockerImage     string `json:"docker_image"`
	DockerTag       string `json:"docker_tag"`
}

type updateClusterPayload struct {
	Name            *string `json:"name,omitempty"`
	ClusterProvider *string `json:"cluster_provider,omitempty"`
	Region          *string `json:"region,omitempty"`
	DockerImage     *string `json:"docker_image,omitempty"`
	DockerTag       *string `json:"docker_tag,omitempty"`
}

// cluster represents dto.ClusterResponse (subset of fields we care about in Terraform).
type cluster struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ClusterProvider string `json:"cluster_provider"`
	Region          string `json:"region"`
	Status          string `json:"status"`
	LastError       string `json:"last_error"`
	RandomToken     string `json:"random_token"`
	CertificateKey  string `json:"certificate_key"`
	DockerImage     string `json:"docker_image"`
	DockerTag       string `json:"docker_tag"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`

	CaptainDomain *struct {
		ID         string `json:"id"`
		DomainName string `json:"domain_name"`
	} `json:"captain_domain"`

	ControlPlaneRecordSet *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"control_plane_record_set"`

	ControlPlaneFQDN *string `json:"control_plane_fqdn"`

	AppsLoadBalancer *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"apps_load_balancer"`

	GlueOpsLoadBalancer *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"glueops_load_balancer"`

	BastionServer *struct {
		ID       string `json:"id"`
		Hostname string `json:"hostname"`
	} `json:"bastion_server"`

	NodePools []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"node_pools"`
}

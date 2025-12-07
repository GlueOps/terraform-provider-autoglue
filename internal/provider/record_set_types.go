package provider

import "encoding/json"

type createRecordSetPayload struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	TTL    *int     `json:"ttl,omitempty"`
	Values []string `json:"values,omitempty"`
}

type updateRecordSetPayload struct {
	Name   *string   `json:"name,omitempty"`
	Type   *string   `json:"type,omitempty"`
	TTL    *int      `json:"ttl,omitempty"`
	Values *[]string `json:"values,omitempty"`
	Status *string   `json:"status,omitempty"`
}

type recordSet struct {
	ID          string          `json:"id"`
	DomainID    string          `json:"domain_id"`
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	TTL         *int            `json:"ttl,omitempty"`
	Values      json.RawMessage `json:"values"` // []string JSON
	Fingerprint string          `json:"fingerprint"`
	Status      string          `json:"status"`
	LastError   string          `json:"last_error"`
	Owner       string          `json:"owner"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

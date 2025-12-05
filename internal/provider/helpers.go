package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

func stringPointerFromAttr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	if s == "" {
		return nil
	}
	return &s
}

package main

import (
	"context"
	"flag"

	"terraform-provider-autoglue/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var (
	version = "0.10.0"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "terraform.gpkg.io/glueops/autoglue", //"registry.terraform.io/glueops/autoglue",
		Debug:   debug,
	})
}

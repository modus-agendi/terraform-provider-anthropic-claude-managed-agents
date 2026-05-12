package main

import (
	"context"
	"flag"
	"log"

	"github.com/asvirida/terraform-provider-claude-managed-agents/internal/provider"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/asvirida/claude-managed-agents",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version, commit), opts); err != nil {
		log.Fatal(err.Error())
	}
}

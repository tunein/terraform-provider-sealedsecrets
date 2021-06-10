package main

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/tunein/terraform-provider-sealedsecrets/util/sh"
	"os"
)

type ProviderConfig struct {
	kubectl string
	kubeseal string
	kubecontext string
}

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"kubeseal": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     sh.Which("kubeseal"),
			},
			"kubectl": {
				Type: schema.TypeString,
				Optional: true,
				Default: sh.Which("kubectl"),
			},
			"server_address": {
				Type:        schema.TypeString,
				Required:    true,
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"sealedsecrets_sealed_secret": resourceSealedSecret(),
		},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	kubeseal := d.Get("kubeseal").(string)
	if !PathExists(kubeseal) {
		diags = append(diags, diag.Diagnostic{
			Severity:      diag.Error,
			Summary:       fmt.Sprintf("kubeseal command doesn't exist at %s", kubeseal),
			Detail:        "",
			AttributePath: nil,
		})

		return nil, diags
	}

	kubectl := d.Get("kubectl").(string)
	if !PathExists(kubectl) {
		diags = append(diags, diag.Diagnostic{
			Severity:      diag.Error,
			Summary:       fmt.Sprintf("kubectl command doesn't exist at %s", kubectl),
			Detail:        "",
			AttributePath: nil,
		})

		return nil, diags
	}


	kClient := KubeClient{kubectl_bin: kubectl}
	config, err := kClient.findContext(ctx, d.Get("server_address").(string))
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary: err.Error(),
		})

		return nil, diags
	}

	return &ProviderConfig{
		kubeseal:     kubeseal,
		kubectl:      kubectl,
		kubecontext:  config.Context.Name,
	}, nil
}

func PathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// doesn't exist
		return false
	}

	return true
}

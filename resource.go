package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/tunein/terraform-provider-sealedsecrets/util/sh"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"strings"
	"time"
)

func resourceSealedSecret() *schema.Resource {
	return &schema.Resource{
		ReadContext:   resourceSealedSecretRead,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
			},
			"scope": {
				Type: schema.TypeString,
				Required: true,
			},
			"type": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "Opaque",
			},
			"labels": {
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"annotations": {
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"data": {
				Type:        schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required:    true,
				Sensitive: true,
			},
			"sha256": {
				Type: schema.TypeString,
				Computed: true,
			},
			"manifest": {
				Type: schema.TypeString,
				Computed: true,
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Read:    schema.DefaultTimeout(1 * time.Minute),
		},
	}
}

func resourceSealedSecretRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	providerConfig := m.(*ProviderConfig)

	var diags diag.Diagnostics

	//var oldDataSha256 string
	//{
	//	if attr, ok := d.GetOk("sha256"); ok {
	//		oldDataSha256 = attr.(string)
	//	}
	//}

	data := make(map[string]string)
	{
		if attr, ok := d.GetOk("data"); ok {
			for key, val := range attr.(map[string]interface{}) {
				data[key] = val.(string)
			}
		}
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity:      diag.Error,
			Summary:       "Failed to marshal secret data",
			Detail:        "",
			AttributePath: nil,
		})

		return diags
	}
	h := sha256.New()
	h.Write(dataBytes)

	//newDataSha := string(h.Sum(nil))

	dataMapBytes := make(map[string][]byte)
	{
		for key, value := range data {
			dataMapBytes[key] = []byte(value)
		}
	}

	// special treatment of the data value
	// we set it to empty so it doesn't persist to state
	d.Set("data", "")

	var secret corev1.Secret

	{
		secret = corev1.Secret{
			TypeMeta:   v1.TypeMeta{
				Kind:       "Secret",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: v1.ObjectMeta{
				Name: d.Get("name").(string),
			},
			Data:       dataMapBytes,
			Type:       corev1.SecretType(d.Get("type").(string)),
		}

		labelsInterfaceMap := d.Get("labels").(map[string]interface{})
		labels := make(map[string]string)
		for key, value := range labelsInterfaceMap {
			labels[key] = value.(string)
		}
		secret.Labels = labels

		annotationsInterfaceMap := d.Get("annotations").(map[string]interface{})
		annotations := make(map[string]string)
		for key, value := range annotationsInterfaceMap {
			annotations[key] = value.(string)
		}
		secret.Annotations = annotations
	}

	secretJsonBytes, err := json.Marshal(secret)
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity:      diag.Error,
			Summary:       "Failed to marshal secret",
			Detail:        "",
			AttributePath: nil,
		})

		return diags
	}

	var sealedSecretBuf bytes.Buffer
	var kubesealStdErr bytes.Buffer

	exitCode, err := sh.Run(ctx, providerConfig.kubeseal, func(o *sh.RunOptions) {
		o.Stdin = strings.NewReader(string(secretJsonBytes))
		o.Stdout = &sealedSecretBuf
		o.Stderr = &kubesealStdErr

		o.Args = []string{
			"--scope", d.Get("scope").(string),
			"--context", providerConfig.kubecontext,
			"--format", "json",
		}

		if attr, ok := d.GetOk("namespace"); ok {
			o.Args = append(o.Args, "--namespace")
			o.Args = append(o.Args, attr.(string))
		}
	})

	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity:      diag.Error,
			Summary:       "Failed to start kubeseal process",
			Detail:        "",
			AttributePath: nil,
		})

		return diags
	}

	if exitCode != 0 {
		diags = append(diags, diag.Diagnostic{
			Severity:      diag.Error,
			Summary:       "Kubeseal failed",
			Detail:        kubesealStdErr.String(),
			AttributePath: nil,
		})

		return diags
	}

	if err := d.Set("manifest", sealedSecretBuf.String()); err != nil {
		return diag.FromErr(err)
	}

	//if newDataSha != oldDataSha256 {
	//	d.SetId("")
	//	return diags
	//}

	d.SetId(strconv.FormatInt(time.Now().Unix(), 10))

	return diags
}

func SHA256(src string) string {
	h := sha256.New()
	h.Write([]byte(src))
	return fmt.Sprintf("%x", h.Sum(nil))
}

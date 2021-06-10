package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/tunein/terraform-provider-sealedsecrets/util/sh"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"strings"
)

const (
	ATTR_NAME = "name"
	ATTR_NAMESPACE = "namespace"
	ATTR_SCOPE = "scope"
	ATTR_TYPE = "type"
	ATTR_LABELS = "labels"
	ATTR_ANNOTATIONS = "annotations"
	ATTR_DATA = "data"
	ATTR_SHA256 = "sha256"
	ATTR_MANIFEST = "manifest"
)

func resourceSealedSecret() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSealedSecretCreate,
		ReadContext:   resourceSealedSecretRead,
		DeleteContext: resourceSealedSecretDelete,
		Schema: map[string]*schema.Schema{
			ATTR_NAME: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew: true,
			},
			ATTR_NAMESPACE: {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew: true,
			},
			ATTR_SCOPE: {
				Type: schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			ATTR_TYPE: {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "Opaque",
				ForceNew: true,
			},
			ATTR_LABELS: {
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
				ForceNew: true,
			},
			ATTR_ANNOTATIONS: {
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
				ForceNew: true,
			},
			ATTR_DATA: {
				Type:        schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required: true,
				ForceNew: true,
				Sensitive:   true,
			},
			ATTR_SHA256: {
				Type: schema.TypeString,
				Computed: true,
			},
			ATTR_MANIFEST: {
				Type: schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceSealedSecretCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	log.Println("SEALEDSECRET CREATE CALLED")
	secretJsonBytes, err := generateSecret(d)

	providerConfig := m.(*ProviderConfig)

	if err != nil {
		return diag.FromErr(err)
	}

	manifest, err := sealIt(
		ctx,
		secretJsonBytes,
		d.Get(ATTR_SCOPE).(string),
		d.Get(ATTR_NAMESPACE).(string),
		providerConfig.kubeseal,
		providerConfig.kubecontext,
	)

	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set(ATTR_MANIFEST, manifest); err != nil {
		return diag.FromErr(err)
	}

	// set this to empty to ensure it's not persisted in state
	//if err := d.Set(ATTR_DATA, make(map[string]interface{})); err != nil {
	//	return diag.FromErr(err)
	//}

	return resourceSealedSecretRead(ctx, d, m)
}

func resourceSealedSecretRead(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
	log.Println("SEALEDSECRET READ CALLED")
	var diags diag.Diagnostics

	dataOld, dataNew := d.GetChange(ATTR_DATA)
	log.Printf("[DEBUG] data old:\n%+v\ndata new:\n%+v\n", dataOld, dataNew)

	secretJsonBytes, err := generateSecret(d)

	log.Printf("[DEBUG] d.Get(ATTR_DATA) %+v", d.Get(ATTR_DATA).(map[string]interface{}))

	if err != nil {
		return diag.FromErr(err)
	}

	// special treatment of the data value
	// we set it to empty so it doesn't persist to state
	//if err := d.Set(ATTR_DATA, map[string]interface{}{}); err != nil {
	//	return diag.FromErr(err)
	//}

	h := sha256.New()
	h.Write(secretJsonBytes)
	newSecretSha := hex.EncodeToString(h.Sum(nil))

	log.Printf("[DEBUG] new sha: %s", newSecretSha)

	d.SetId(newSecretSha)
	if err := d.Set(ATTR_SHA256, newSecretSha); err != nil {
		return diag.FromErr(err)
	}

	//if d.HasChange(ATTR_SHA256) {
	//	d.SetId("")
	//}

	//oldManifest, newManifest := d.GetChange(ATTR_MANIFEST)
	//if d.HasChange(ATTR_SHA256) {
	//	if err := d.Set(ATTR_MANIFEST, newManifest); err != nil {
	//		return diag.FromErr(err)
	//	}
	//} else {
	//	if err := d.Set(ATTR_MANIFEST, oldManifest); err != nil {
	//		return diag.FromErr(err)
	//	}
	//}

	return diags
}

func resourceSealedSecretDelete(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	log.Println("SEALEDSECRET DELETE CALLED")
	var diags diag.Diagnostics

	//if err := d.Set(ATTR_DATA, map[string]interface{}{}); err != nil {
	//	return diag.FromErr(err)
	//}
	//d.SetId("")

	return diags
}

func interfaceMapToStringMap(input map[string]interface{}) map[string]string {
	output := make(map[string]string)
	for key, value := range input {
		output[key] = value.(string)
	}

	return output
}

func sealIt(ctx context.Context, secretJsonBytes []byte, scope, namespace, kubeseal_bin, kubecontext string) (string, error) {
	var sealedSecretBuf bytes.Buffer
	var kubesealStdErr bytes.Buffer

	exitCode, err := sh.Run(ctx, kubeseal_bin, func(o *sh.RunOptions) {
		o.Stdin = strings.NewReader(string(secretJsonBytes))
		o.Stdout = &sealedSecretBuf
		o.Stderr = &kubesealStdErr

		o.Args = []string{
			"--scope", scope,
			"--context", kubecontext,
			"--format", "json",
		}

		if namespace != "" {
			o.Args = append(o.Args, "--namespace")
			o.Args = append(o.Args, namespace)
		}
	})

	if err != nil {
		return "", fmt.Errorf("Failed to start kubeseal process: %w", err)
	}

	if exitCode != 0 {
		return "", errors.New(fmt.Sprintf("Kubeseal failed: %s", kubesealStdErr.String()))
	}

	return sealedSecretBuf.String(), nil
}

func generateSecret(d *schema.ResourceData) ([]byte, error) {
	dataInterfaceMap := d.Get(ATTR_DATA).(map[string]interface{})
	data := make(map[string][]byte, len(dataInterfaceMap))
	for key, val := range dataInterfaceMap {
		valString := val.(string)
		valBytes := make([]byte, base64.StdEncoding.EncodedLen(len(valString)))
		log.Printf("[DEBUG] data value: %s", valString)
		base64.StdEncoding.Encode(valBytes, []byte(valString))
		data[key] = valBytes
	}

	labels := interfaceMapToStringMap(d.Get(ATTR_LABELS).(map[string]interface{}))
	annotations := interfaceMapToStringMap(d.Get(ATTR_ANNOTATIONS).(map[string]interface{}))

	secret := corev1.Secret{
		TypeMeta:   v1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name: d.Get(ATTR_NAME).(string),
			Labels: labels,
			Annotations: annotations,
		},
		Data:       data,
		Type:       corev1.SecretType(d.Get(ATTR_TYPE).(string)),
	}

	secretJsonBytes, err := json.Marshal(secret)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to marshal secret to json: %w", err)
	}

	return secretJsonBytes, nil
}

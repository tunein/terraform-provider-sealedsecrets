/*
Copyright 2016 The Tanka Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tunein/terraform-provider-sealedsecrets/util/sh"
	"strings"

	"github.com/stretchr/objx"
	funk "github.com/thoas/go-funk"
)

// Config represents a single KUBECONFIG entry (single context + cluster)
// Omits the arrays of the original schema, to ease using.
type Config struct {
	Cluster Cluster `json:"cluster"`
	Context KubeContext `json:"context"`
}

// Context is a kubectl context
type KubeContext struct {
	Context struct {
		Cluster   string `json:"cluster"`
		User      string `json:"user"`
		Namespace string `json:"namespace"`
	} `json:"context"`
	Name string `json:"name"`
}

// Cluster is a kubectl cluster
type Cluster struct {
	Cluster struct {
		Server string `json:"server"`
	} `json:"cluster"`
	Name string `json:"name"`
}

type KubeClient struct {
	kubectl_bin string
}

// findContext returns a valid context from $KUBECONFIG that uses the given
// apiServer endpoint.
func (k *KubeClient) findContext(ctx context.Context, endpoint string) (Config, error) {
	cluster, kubeContext, err := k.ContextFromIP(ctx, endpoint)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Context: *kubeContext,
		Cluster: *cluster,
	}, nil
}

// Kubeconfig returns the merged $KUBECONFIG of the host
func (k *KubeClient) Kubeconfig(ctx context.Context) (objx.Map, error) {
	cfgJSON := bytes.Buffer{}

	kubectlStderr := bytes.Buffer{}

	code, err := sh.Run(ctx, k.kubectl_bin, func(o *sh.RunOptions) {
		o.Args = []string{
			"config",
			"view",
			"-o", "json",
		}

		o.Stdout = &cfgJSON
		o.Stderr = &kubectlStderr
	})

	if err != nil {
		return nil, fmt.Errorf("unable to start process: %w", err)
	}

	if code != 0 {
		return nil, errors.New(fmt.Sprintf("failed to get kubeconfig: %s", kubectlStderr.String()))
	}

	return objx.FromJSON(cfgJSON.String())
}

// Contexts returns a list of context names
func (k *KubeClient) Contexts(ctx context.Context) ([]string, error) {
	kubeContexts := bytes.Buffer{}

	kubectlStderr := bytes.Buffer{}

	code, err := sh.Run(ctx, k.kubectl_bin, func(o *sh.RunOptions) {
		o.Args = []string{
			"config",
			"get-contexts",
			"-o=name",
		}

		o.Stdout = &kubeContexts
		o.Stderr = &kubectlStderr
	})

	if err != nil {
		return nil, fmt.Errorf("unable to start kubectl process: %w", err)
	}

	if code != 0 {
		return nil, errors.New(fmt.Sprintf("failed to get kube contexts: %s", kubectlStderr.String()))
	}

	return strings.Split(kubeContexts.String(), "\n"), nil
}

// ContextFromIP searches the $KUBECONFIG for a context using a cluster that matches the apiServer
func (k *KubeClient) ContextFromIP(ctx context.Context, apiServer string) (*Cluster, *KubeContext, error) {
	cfg, err := k.Kubeconfig(ctx)
	if err != nil {
		return nil, nil, err
	}

	// find the correct cluster
	var cluster Cluster
	clusters, err := tryMSISlice(cfg.Get("clusters"), "clusters")
	if err != nil {
		return nil, nil, err
	}

	err = find(clusters, "cluster.server", apiServer, &cluster)
	if err == ErrorNoMatch {
		return nil, nil, ErrorNoCluster(apiServer)
	} else if err != nil {
		return nil, nil, err
	}

	// find a context that uses the cluster
	var context KubeContext
	contexts, err := tryMSISlice(cfg.Get("contexts"), "contexts")
	if err != nil {
		return nil, nil, err
	}

	err = find(contexts, "context.cluster", cluster.Name, &context)
	if err == ErrorNoMatch {
		return nil, nil, ErrorNoContext(cluster.Name)
	} else if err != nil {
		return nil, nil, err
	}

	return &cluster, &context, nil
}

// IPFromContext parses $KUBECONFIG, finds the cluster with the given name and
// returns the cluster's endpoint
func (k *KubeClient) IPFromContext(ctx context.Context, name string) (ip string, err error) {
	cfg, err := k.Kubeconfig(ctx)
	if err != nil {
		return "", err
	}

	// find a context with the given name
	var kubeContext KubeContext
	contexts, err := tryMSISlice(cfg.Get("contexts"), "contexts")
	if err != nil {
		return "", err
	}

	err = find(contexts, "name", name, &kubeContext)
	if err == ErrorNoMatch {
		return "", ErrorNoContext(name)
	} else if err != nil {
		return "", err
	}

	// find the cluster of the context
	var cluster Cluster
	clusters, err := tryMSISlice(cfg.Get("clusters"), "clusters")
	if err != nil {
		return "", err
	}

	clusterName := kubeContext.Context.Cluster
	err = find(clusters, "name", clusterName, &cluster)
	if err == ErrorNoMatch {
		return "", fmt.Errorf("no cluster named `%s` as required by context `%s` was found. Please check your $KUBECONFIG", clusterName, name)
	} else if err != nil {
		return "", err
	}

	return cluster.Cluster.Server, nil
}

func tryMSISlice(v *objx.Value, what string) ([]map[string]interface{}, error) {
	if s := v.MSISlice(); s != nil {
		return s, nil
	}

	data, ok := v.Data().([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected %s to be of type `[]map[string]interface{}`, but got `%T` instead", what, v.Data())
	}
	return data, nil
}

// ErrorNoMatch occurs when no item matched had the expected value
var ErrorNoMatch error = errors.New("no matches found")

// find attempts to find an object in list whose prop equals expected.
// If found, the value is unmarshalled to ptr, otherwise errNotFound is returned.
func find(list []map[string]interface{}, prop string, expected string, ptr interface{}) error {
	var findErr error
	i := funk.Find(list, func(x map[string]interface{}) bool {
		if findErr != nil {
			return false
		}

		got := objx.New(x).Get(prop).Data()
		str, ok := got.(string)
		if !ok {
			findErr = fmt.Errorf("testing whether `%s` is `%s`: unable to parse `%v` as string", prop, expected, got)
			return false
		}

		return str == expected
	})
	if findErr != nil {
		return findErr
	}

	if i == nil {
		return ErrorNoMatch
	}

	o := objx.New(i).MustJSON()
	return json.Unmarshal([]byte(o), ptr)
}

// ErrorNoCluster means that the cluster that was searched for couldn't be found
type ErrorNoCluster string

func (e ErrorNoCluster) Error() string {
	return fmt.Sprintf("no cluster that matches the apiServer `%s` was found. Please check your $KUBECONFIG", string(e))
}

// ErrorNoContext means that the context that was searched for couldn't be found
type ErrorNoContext string

func (e ErrorNoContext) Error() string {
	return fmt.Sprintf("no context named `%s` was found. Please check your $KUBECONFIG", string(e))
}

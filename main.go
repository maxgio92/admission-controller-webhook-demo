/*
Copyright (c) 2019 StackRox Inc.

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
	"errors"
	"flag"
	"fmt"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
	"path/filepath"

	admission "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tlsDir              = `/run/secrets/tls`
	tlsCertFile         = `tls.crt`
	tlsKeyFile          = `tls.key`
	configMapNamePrefix = "impersonation-shell-admin-kubeconfig-"
	proxyServerURL      = "https://capsule-proxy.capsule-system.svc:9001"
)

var (
	configMapResource             = metav1.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	optionConfigMapNamePrefix     *string
	optionConfigMapKey            *string
	optionProxyServerURL          *string
	optionForceValidateKubeconfig *bool
)

// applyProxyServer implements the logic of the admission controller webhook.
// For every ConfigMap that is created it first checks that it contains a kubeconfig.
// If it contains a valid kubeconfig, all the clusters URLs are replaced with the one of the proxy.
func applyProxyServer(req *admission.AdmissionRequest) ([]patchOperation, error) {
	// This handler should only get called on ConfigMap objects as per the MutatingWebhookConfiguration in the YAML file.
	// However, if (for whatever reason) this gets invoked on an object of a different kind, issue a log message but
	// let the object request pass through otherwise.
	if req.Resource != configMapResource {
		log.Printf("expect resource to be %s", configMapResource)
		return nil, nil
	}

	// Parse the ConfigMap object.
	raw := req.Object.Raw
	configMap := corev1.ConfigMap{}
	if _, _, err := universalDeserializer.Decode(raw, nil, &configMap); err != nil {
		return nil, fmt.Errorf("could not deserialize configMap object: %v", err)
	}

	// Create patch operations to apply proxy server to the kubeconfig.
	var patches []patchOperation

	if _, ok := configMap.Data[*optionConfigMapKey]; !ok {
		// The ConfigMap does not contain a kubeconfig config key.
		return nil, errors.New("the configmap does not contain a key with name 'config'")
	}

	kubeconfig, err := clientcmd.Load([]byte(configMap.Data["config"]))
	if err != nil {
		// The ConfigMap does not contain a valid kubeconfig.
		return nil, fmt.Errorf("error when loading client cmd config: %v", err)
	}

	// Overwrite all the cluster server URLs in the kubeconifg.
	for _, cluster := range kubeconfig.Clusters {
		cluster.Server = *optionProxyServerURL
	}

	if *optionForceValidateKubeconfig {
		if err := clientcmd.Validate(*kubeconfig); err != nil {
			// The built kubeconfig is not valid.
			return nil, fmt.Errorf("error validating the kubeconfig: %v", err)
		}
	}

	kubeconfigBuffer, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return nil, err
	}

	patches = append(patches, patchOperation{
		Op:    "replace",
		Path:  "/data/config",
		Value: fmt.Sprintf("%s", kubeconfigBuffer),
	})

	return patches, nil
}

func main() {
	certPath := flag.String("cert-path", filepath.Join(tlsDir, tlsCertFile), "--cert-path=/path/to/tls.crt")
	keyPath := flag.String("key-path", filepath.Join(tlsDir, tlsKeyFile), "--cert-path=/path/to/tls.crt")
	optionConfigMapNamePrefix = flag.String("configmap-prefix", configMapNamePrefix, "--configmap-prefix=<prefix>")
	optionConfigMapKey = flag.String("configmap-key", "config", "--configmap-key=<key>")
	optionProxyServerURL = flag.String("proxy-server", proxyServerURL, "--proxy-server=<URL>")
	optionForceValidateKubeconfig = flag.Bool("force-validate", false, "--force-validate")

	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle("/mutate", admitFuncHandler(applyProxyServer))
	server := &http.Server{
		// We listen on port 8443 such that we do not need root privileges or extra capabilities for this server.
		// The Service object will take care of mapping this port to the HTTPS port 443.
		Addr:    ":8443",
		Handler: mux,
	}
	log.Fatal(server.ListenAndServeTLS(*certPath, *keyPath))
}

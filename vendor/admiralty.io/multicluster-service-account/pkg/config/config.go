/*
Copyright 2018 The Multicluster-Service-Account Authors.

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

package config // import "admiralty.io/multicluster-service-account/pkg/config"

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang/glog"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/cert"
)

var saiDir = "/var/run/secrets/admiralty.io/serviceaccountimports/"

// ConfigAndNamespace returns a rest.Config and namespace from a mounted service account import.
// If several service account imports are mounted, an error is returned. It no service account import
// is mounted, ConfigAndNamespace falls back to the current kubeconfig context or the regular
// service account.
func ConfigAndNamespace() (*rest.Config, string, error) {
	return NamedConfigAndNamespace("")
}

// NamedConfigAndNamespace returns a rest.Config and namespace from a mounted service account import
// whose name is given as an argument. It no mounted service account import matches that name,
// NamedConfigAndNamespace attempts to fall back to an eponymous kubeconfig context. Otherwise,
// an error is returned.
func NamedConfigAndNamespace(name string) (*rest.Config, string, error) {
	cfg, ns, err := NamedServiceAccountImportConfigAndNamespace(name)
	if err == nil {
		return cfg, ns, nil
	}
	return ConfigAndNamespaceForContext(name)
}

// AllNamedConfigsAndNamespaces returns a map of rest.Configs and namespaces corresponding to
// the mounted service account imports and kubeconfig contexts. If none is found, the map is empty.
func AllNamedConfigsAndNamespaces() (map[string]ConfigAndNamespaceTuple, error) {
	out, err := AllServiceAccountImportConfigsAndNamespaces()
	if err != nil {
		return nil, err
	}
	// add config and namespace for each kubeconfig context
	// HACK: there may be a better way
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	raw, err := loader.RawConfig()
	if err != nil {
		return nil, err
	}
	for ctx := range raw.Contexts { // loop over context names, disregard content and reload with name as override
		cfg, ns, err := ConfigAndNamespaceForContext(ctx)
		if err != nil {
			return nil, err
		}
		out[ctx] = ConfigAndNamespaceTuple{Config: cfg, Namespace: ns}
	}
	return out, nil
}

// ConfigAndNamespaceForContext returns a rest.Config and namespace from a kubeconfig context
// whose name is given as an argument. It it doesn't exist, an error is returned.
func ConfigAndNamespaceForContext(context string) (*rest.Config, string, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{CurrentContext: context}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	return configAndNamespace(loader)
}

// ServiceAccountImportConfigAndNamespace returns a rest.Config and namespace from a mounted service account import.
// If none or several service account imports are mounted, an error is returned.
func ServiceAccountImportConfigAndNamespace() (*rest.Config, string, error) {
	return NamedServiceAccountImportConfigAndNamespace("")
}

// NamedServiceAccountImportConfigAndNamespace returns a rest.Config and namespace from a mounted service account import
// whose name is given as an argument. It no mounted service account import matches that name,
// an error is returned.
func NamedServiceAccountImportConfigAndNamespace(name string) (*rest.Config, string, error) {
	l, err := NewNamedServiceAccountImportConfigLoader(name)
	if err != nil {
		return nil, "", err
	}
	return l.ConfigAndNamespace()
}

// ConfigAndNamespaceTuple combines a rest.Config pointer and Namespace. It is used as a map value in
// functions of this package returning multiple configurations (the "All...()" functions).
type ConfigAndNamespaceTuple struct {
	Config    *rest.Config
	Namespace string
}

// AllServiceAccountImportConfigsAndNamespaces returns a map of rest.Configs and namespaces corresponding to
// the mounted service account imports. If none is found, the map is empty.
func AllServiceAccountImportConfigsAndNamespaces() (map[string]ConfigAndNamespaceTuple, error) {
	lm, err := AllServiceAccountImportConfigLoaders()
	if err != nil {
		return nil, err
	}
	out := make(map[string]ConfigAndNamespaceTuple)
	for sai, l := range lm {
		cfg, ns, err := l.ConfigAndNamespace()
		if err != nil {
			return nil, err // TODO? partial errors
		}
		out[sai] = ConfigAndNamespaceTuple{Config: cfg, Namespace: ns}
	}
	return out, nil
}

// ServiceAccountImportConfigLoader partially implements client-go/tools/clientcmd.ClientConfig,
// to integrate with that package's general-purpose config loading logic.
type ServiceAccountImportConfigLoader struct {
	Name      string
	config    *rest.Config
	namespace string
}

func NewServiceAccountImportConfigLoader() (*ServiceAccountImportConfigLoader, error) {
	n, err := SingleServiceAccountImportName()
	if err != nil {
		return nil, err
	}
	return &ServiceAccountImportConfigLoader{Name: n}, nil
}

func NewNamedServiceAccountImportConfigLoader(name string) (*ServiceAccountImportConfigLoader, error) {
	if name == "" {
		return NewServiceAccountImportConfigLoader()
	}
	mounted, err := ServiceAccountImportMounted(name)
	if err != nil {
		return nil, err
	}
	if !mounted {
		return nil, fmt.Errorf("service account import %s not mounted", name)
	}
	return &ServiceAccountImportConfigLoader{Name: name}, nil
}

func AllServiceAccountImportConfigLoaders() (map[string]*ServiceAccountImportConfigLoader, error) {
	sais, err := AllServiceAccountImportNames()
	if err != nil {
		return nil, err
	}
	saics := make(map[string]*ServiceAccountImportConfigLoader)
	for _, sai := range sais {
		saics[sai] = &ServiceAccountImportConfigLoader{Name: sai}
	}
	return saics, nil
}

func AllServiceAccountImportNames() ([]string, error) {
	f, err := os.Open(saiDir)
	if err != nil {
		// most likely saiDir doesn't exist, which means there are no service account imports
		return []string{}, nil
	}
	defer f.Close()
	fileInfo, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}
	var sais []string
	for _, file := range fileInfo {
		if file.IsDir() {
			sais = append(sais, file.Name())
		}
	}
	return sais, nil
}

func SingleServiceAccountImportName() (string, error) {
	sais, err := AllServiceAccountImportNames()
	if err != nil {
		return "", err
	}

	if len(sais) == 0 {
		return "", fmt.Errorf("no service account import mounted")
	}

	if len(sais) > 1 {
		return "", fmt.Errorf("multiple service account import mounted, pick one")
	}

	return sais[0], nil
}

func ServiceAccountImportMounted(name string) (bool, error) {
	_, err := os.Stat(saiDir + name)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (l *ServiceAccountImportConfigLoader) ClientConfig() (*rest.Config, error) {
	if l.config != nil {
		return l.config, nil
	}

	basePath := saiDir + l.Name
	server, err := ioutil.ReadFile(basePath + "/server")
	if err != nil {
		return nil, err
	}
	token, err := ioutil.ReadFile(basePath + "/token")
	if err != nil {
		return nil, err
	}
	tlsClientConfig := rest.TLSClientConfig{}
	rootCAFile := basePath + "/ca.crt"
	if _, err := cert.NewPool(rootCAFile); err != nil {
		glog.Errorf("Expected to load root CA config from %s, but got err: %v", rootCAFile, err)
	} else {
		tlsClientConfig.CAFile = rootCAFile
	}

	return &rest.Config{
		Host:            string(server),
		BearerToken:     string(token),
		TLSClientConfig: tlsClientConfig,
	}, nil
}

func (l *ServiceAccountImportConfigLoader) Namespace() (string, bool, error) {
	if l.namespace != "" {
		return l.namespace, false, nil
	}

	basePath := saiDir + l.Name
	ns, err := ioutil.ReadFile(basePath + "/namespace")
	if err != nil {
		return "", false, err
	}
	return string(ns), false, nil
}

func (l *ServiceAccountImportConfigLoader) RawConfig() (clientcmdapi.Config, error) {
	panic("not implemented")
}

func (l *ServiceAccountImportConfigLoader) ConfigAccess() clientcmd.ConfigAccess {
	panic("not implemented")
}

func (l *ServiceAccountImportConfigLoader) ConfigAndNamespace() (*rest.Config, string, error) {
	return configAndNamespace(l)
}

func configAndNamespace(loader clientcmd.ClientConfig) (*rest.Config, string, error) {
	cfg, err := loader.ClientConfig()
	if err != nil {
		return nil, "", err
	}
	ns, _, err := loader.Namespace()
	if err != nil {
		return nil, "", err
	}
	return cfg, ns, nil
}

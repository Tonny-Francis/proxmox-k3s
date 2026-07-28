package k3s

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type kubeConfig struct {
	APIVersion     string                   `yaml:"apiVersion"`
	Kind           string                   `yaml:"kind"`
	Clusters       []kubeConfigCluster      `yaml:"clusters"`
	Contexts       []kubeConfigContext       `yaml:"contexts"`
	Users          []kubeConfigUser         `yaml:"users"`
	CurrentContext string                   `yaml:"current-context"`
}

type kubeConfigCluster struct {
	Name    string `yaml:"name"`
	Cluster struct {
		Server                   string `yaml:"server"`
		CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	} `yaml:"cluster"`
}

type kubeConfigContext struct {
	Name    string `yaml:"name"`
	Context struct {
		Cluster string `yaml:"cluster"`
		User    string `yaml:"user"`
	} `yaml:"context"`
}

type kubeConfigUser struct {
	Name string      `yaml:"name"`
	User interface{} `yaml:"user"`
}

func RewriteKubeconfig(rawKubeconfig []byte, clusterName, vip string) ([]byte, error) {
	var kc kubeConfig
	if err := yaml.Unmarshal(rawKubeconfig, &kc); err != nil {
		return nil, fmt.Errorf("parsing kubeconfig: %w", err)
	}

	newServer := fmt.Sprintf("https://%s:6443", vip)
	for i := range kc.Clusters {
		kc.Clusters[i].Cluster.Server = newServer
		kc.Clusters[i].Name = clusterName
	}
	for i := range kc.Contexts {
		kc.Contexts[i].Name = clusterName
		kc.Contexts[i].Context.Cluster = clusterName
		kc.Contexts[i].Context.User = clusterName + "-admin"
	}
	for i := range kc.Users {
		kc.Users[i].Name = clusterName + "-admin"
	}
	kc.CurrentContext = clusterName

	return yaml.Marshal(kc)
}

func MergeKubeconfig(newKubeconfig []byte, targetPath string) error {
	if targetPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		targetPath = filepath.Join(home, ".kube", "config")
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
		return err
	}

	existing, err := os.ReadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading existing kubeconfig: %w", err)
	}

	if len(existing) == 0 {
		return os.WriteFile(targetPath, newKubeconfig, 0600)
	}

	var existingKC, newKC kubeConfig
	if err := yaml.Unmarshal(existing, &existingKC); err != nil {
		return fmt.Errorf("parsing existing kubeconfig: %w", err)
	}
	if err := yaml.Unmarshal(newKubeconfig, &newKC); err != nil {
		return fmt.Errorf("parsing new kubeconfig: %w", err)
	}

	merged := mergeConfigs(existingKC, newKC)
	data, err := yaml.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshaling merged kubeconfig: %w", err)
	}

	return os.WriteFile(targetPath, data, 0600)
}

func mergeConfigs(base, extra kubeConfig) kubeConfig {
	result := base

	for _, c := range extra.Clusters {
		if !clusterExists(result.Clusters, c.Name) {
			result.Clusters = append(result.Clusters, c)
		} else {
			for i := range result.Clusters {
				if result.Clusters[i].Name == c.Name {
					result.Clusters[i] = c
				}
			}
		}
	}

	for _, ctx := range extra.Contexts {
		if !contextExists(result.Contexts, ctx.Name) {
			result.Contexts = append(result.Contexts, ctx)
		}
	}

	for _, u := range extra.Users {
		if !userExists(result.Users, u.Name) {
			result.Users = append(result.Users, u)
		}
	}

	return result
}

func clusterExists(clusters []kubeConfigCluster, name string) bool {
	for _, c := range clusters {
		if c.Name == name {
			return true
		}
	}
	return false
}

func contextExists(contexts []kubeConfigContext, name string) bool {
	for _, c := range contexts {
		if strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

func userExists(users []kubeConfigUser, name string) bool {
	for _, u := range users {
		if u.Name == name {
			return true
		}
	}
	return false
}

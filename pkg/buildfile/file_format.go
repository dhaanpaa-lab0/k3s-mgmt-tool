package buildfile

import (
	"os"

	"gopkg.in/yaml.v3"
)

type SetupFile struct {
	HelmRepos  map[string]string `yaml:"helm_repos"`
	HelmCharts []string          `yaml:"helm_charts"`
	Manifests  []string          `yaml:"k8s_manifests"`
}

func LoadFromFile() (*SetupFile, error) {
	data, err := os.ReadFile("Buildfile.yaml")
	if err != nil {
		return nil, err
	}

	var setup SetupFile
	if err := yaml.Unmarshal(data, &setup); err != nil {
		return nil, err
	}

	return &setup, nil
}

func (s *SetupFile) SaveToFile() error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	return os.WriteFile("Buildfile.yaml", data, 0644)
}

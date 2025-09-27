package buildfile

import (
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// clamp255 ensures the input string is at most 255 Unicode code points.
// It is rune-safe and leaves shorter strings unchanged.
func clamp255(s string) string {
	r := []rune(s)
	if len(r) > 255 {
		return string(r[:255])
	}
	return s
}

type SetupFile struct {
	HelmRepos      map[string]string `yaml:"helm_repos"`
	HelmCharts     map[string]string `yaml:"helm_charts"` // key: release name, value: chart name
	Manifests      []string          `yaml:"k8s_manifests"`
	StartupScripts []string          `yaml:"startup_scripts"`
	GitRepos       map[string]string `yaml:"git_repos"` // key: repo name, value: git URL this is used for being able to setup initial k8s workspaces
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

// EnsureInit ensures maps/slices are non-nil for safe use
func (s *SetupFile) EnsureInit() {
	if s.HelmRepos == nil {
		s.HelmRepos = make(map[string]string)
	}
	if s.HelmCharts == nil {
		s.HelmCharts = make(map[string]string)
	}
	if s.Manifests == nil {
		s.Manifests = []string{}
	}
	if s.StartupScripts == nil {
		s.StartupScripts = []string{}
	}
	if s.GitRepos == nil {
		s.GitRepos = make(map[string]string)
	}
}

// AddHelmRepo adds or updates a Helm repository
func (s *SetupFile) AddHelmRepo(name, url string) {
	s.EnsureInit()
	name = clamp255(strings.TrimSpace(name))
	url = clamp255(strings.TrimSpace(url))
	if name == "" || url == "" {
		return
	}
	s.HelmRepos[name] = url
}

// RemoveHelmRepo removes a Helm repository by name. Returns true if removed.
func (s *SetupFile) RemoveHelmRepo(name string) bool {
	s.EnsureInit()
	if _, ok := s.HelmRepos[name]; ok {
		delete(s.HelmRepos, name)
		return true
	}
	return false
}

// AddHelmChart adds or updates a chart mapping for a release. Returns true if added new, false if updated existing with same value.
func (s *SetupFile) AddHelmChart(release string, chart string) bool {
	s.EnsureInit()
	release = clamp255(strings.TrimSpace(release))
	chart = clamp255(strings.TrimSpace(chart))
	if release == "" || chart == "" {
		return false
	}
	if existing, ok := s.HelmCharts[release]; ok {
		if existing == chart {
			return false
		}
	}
	s.HelmCharts[release] = chart
	return true
}

// RemoveHelmChart removes a chart by release name. Returns true if removed.
func (s *SetupFile) RemoveHelmChart(release string) bool {
	s.EnsureInit()
	if _, ok := s.HelmCharts[release]; ok {
		delete(s.HelmCharts, release)
		return true
	}
	return false
}

// AddManifest adds a manifest path if not present
func (s *SetupFile) AddManifest(path string) bool {
	s.EnsureInit()
	path = clamp255(strings.TrimSpace(path))
	if path == "" {
		return false
	}
	for _, p := range s.Manifests {
		if p == path {
			return false
		}
	}
	s.Manifests = append(s.Manifests, path)
	s.sortUnique()
	return true
}

// RemoveManifest removes a manifest path.
func (s *SetupFile) RemoveManifest(path string) bool {
	s.EnsureInit()
	removed := false
	out := make([]string, 0, len(s.Manifests))
	for _, p := range s.Manifests {
		if p == path {
			removed = true
			continue
		}
		out = append(out, p)
	}
	s.Manifests = out
	return removed
}

// AddStartupScript adds a script path if not present
func (s *SetupFile) AddStartupScript(path string) bool {
	s.EnsureInit()
	path = clamp255(strings.TrimSpace(path))
	if path == "" {
		return false
	}
	for _, p := range s.StartupScripts {
		if p == path {
			return false
		}
	}
	s.StartupScripts = append(s.StartupScripts, path)
	s.sortUnique()
	return true
}

// RemoveStartupScript removes a script path.
func (s *SetupFile) RemoveStartupScript(path string) bool {
	s.EnsureInit()
	removed := false
	out := make([]string, 0, len(s.StartupScripts))
	for _, p := range s.StartupScripts {
		if p == path {
			removed = true
			continue
		}
		out = append(out, p)
	}
	s.StartupScripts = out
	return removed
}

func (s *SetupFile) sortUnique() {
	// keep manifests/scripts sorted to maintain deterministic file order
	s.Manifests = uniqueSorted(s.Manifests)
	s.StartupScripts = uniqueSorted(s.StartupScripts)
}

func uniqueSorted(in []string) []string {
	m := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := m[v]; ok {
			continue
		}
		m[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// AddGitRepo adds or updates a Git repository mapping (name -> URL)
func (s *SetupFile) AddGitRepo(name, url string) bool {
	s.EnsureInit()
	name = clamp255(strings.TrimSpace(name))
	url = clamp255(strings.TrimSpace(url))
	if name == "" || url == "" {
		return false
	}
	if existing, ok := s.GitRepos[name]; ok {
		if existing == url {
			return false
		}
	}
	s.GitRepos[name] = url
	return true
}

// RemoveGitRepo removes a Git repository by name
func (s *SetupFile) RemoveGitRepo(name string) bool {
	s.EnsureInit()
	if _, ok := s.GitRepos[name]; ok {
		delete(s.GitRepos, name)
		return true
	}
	return false
}

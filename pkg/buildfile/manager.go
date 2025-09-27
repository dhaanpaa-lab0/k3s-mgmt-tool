package buildfile

import (
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/repo"
)

// IsRepoExists checks if a repository with the same name and URL already exists
func (s *SetupFile) IsRepoExists(name, url string) bool {
	if existingURL, ok := s.HelmRepos[name]; ok {
		return existingURL == url
	}
	return false
}

// InitHelmClient initializes a new Helm client settings
func InitHelmClient() *cli.EnvSettings {
	settings := cli.New()
	return settings
}

// GetHelmRepoFile returns the repository file for Helm
func GetHelmRepoFile() (*repo.File, error) {
	settings := InitHelmClient()
	return repo.LoadFile(settings.RepositoryConfig)
}

func RepoExistsInHelmLocal(f *repo.File, name string, url string) bool {
	for _, repoInHelm := range f.Repositories {
		if strings.ToUpper(repoInHelm.Name) == strings.ToUpper(name) && strings.ToUpper(repoInHelm.URL) == strings.ToUpper(url) {
			return true
		}
	}
	return false
}

// HelmRepoDifferences represents differences between the SetupFile helm repos and local helm configuration
type HelmRepoDifferences struct {
	MissingInLocal  map[string]string            // present in SetupFile but not in local helm (by name or exact name+url pair)
	MissingInSetup  map[string]string            // present in local helm but not in SetupFile
	URLMismatches   map[string]struct{ SetupURL, LocalURL string } // same name exists in both, but URLs differ
}

// DiffHelmReposWithLocal compares the SetupFile helm repositories with the current local helm repos
// and returns the differences.
func (s *SetupFile) DiffHelmReposWithLocal() (*HelmRepoDifferences, error) {
	f, err := GetHelmRepoFile()
	if err != nil {
		return nil, err
	}

	// Build local map name -> url (case-insensitive compare for names, but return original keys/values)
	localByName := make(map[string]string, len(f.Repositories))
	for _, r := range f.Repositories {
		localByName[strings.ToLower(r.Name)] = r.URL
	}

	diff := &HelmRepoDifferences{
		MissingInLocal: make(map[string]string),
		MissingInSetup: make(map[string]string),
		URLMismatches:  make(map[string]struct{ SetupURL, LocalURL string }),
	}

	// compare: items in setup vs local
	for name, setupURL := range s.HelmRepos {
		lowerName := strings.ToLower(name)
		if localURL, ok := localByName[lowerName]; ok {
			// exists by name, check URL
			if !urlsEqual(setupURL, localURL) {
				diff.URLMismatches[name] = struct{ SetupURL, LocalURL string }{SetupURL: setupURL, LocalURL: localURL}
			}
			continue
		}
		// not found locally
		diff.MissingInLocal[name] = setupURL
	}

	// compare: items in local vs setup
	setupByName := make(map[string]string, len(s.HelmRepos))
	for name, url := range s.HelmRepos {
		setupByName[strings.ToLower(name)] = url
	}
	for _, r := range f.Repositories {
		if _, ok := setupByName[strings.ToLower(r.Name)]; !ok {
			// present locally, not in setup
			diff.MissingInSetup[r.Name] = r.URL
		}
	}

	return diff, nil
}

func urlsEqual(a, b string) bool {
	return strings.TrimRight(strings.ToLower(a), "/") == strings.TrimRight(strings.ToLower(b), "/")
}

// DiffInstalledChartsMissingInBuildfile lists Helm releases deployed in the cluster
// that are not present in the Buildfile's HelmCharts map (keyed by release name).
// It returns a map of releaseName -> chartName (chart metadata name without repo prefix).
func (s *SetupFile) DiffInstalledChartsMissingInBuildfile() (map[string]string, error) {
	settings := InitHelmClient()

	// Initialize Helm action configuration
	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), "", "secrets", func(string, ...interface{}) {}); err != nil {
		return nil, err
	}

	lister := action.NewList(cfg)
	lister.AllNamespaces = true
	lister.Deployed = true // only deployed releases

	releases, err := lister.Run()
	if err != nil {
		return nil, err
	}

	// Build set of release names present in Buildfile
	inBuildfile := make(map[string]struct{})
	for rel := range s.HelmCharts {
		inBuildfile[strings.ToLower(strings.TrimSpace(rel))] = struct{}{}
	}

	missing := make(map[string]string)
	for _, r := range releases {
		relName := strings.TrimSpace(r.Name)
		if relName == "" {
			continue
		}
		if _, ok := inBuildfile[strings.ToLower(relName)]; ok {
			continue
		}
		chartName := ""
		if r.Chart != nil && r.Chart.Metadata != nil {
			chartName = strings.TrimSpace(r.Chart.Metadata.Name)
		}
		if chartName == "" {
			continue
		}
		missing[relName] = chartName
	}

	return missing, nil
}

/*
Copyright © 2025 Daniel Haanpaa <djh@nexus-sds.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"nexus-sites.net/k3s-mgmt-tool/pkg/buildfile"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

var doSyncHelmRepos bool
var doEnsureCharts bool
var doApplyManifests bool
var doRunStartup bool
var doAll bool

// syncToClusterCmd represents the syncToCluster command
var syncToClusterCmd = &cobra.Command{
	Use:   "syncToCluster",
	Short: "Sync local environment and cluster to match Buildfile",
	Long:  "Synchronize Helm repos, ensure Helm charts are installed, apply Kubernetes manifests, and run startup scripts defined in Buildfile.yaml.",
	Run: func(cmd *cobra.Command, args []string) {
		// Load Buildfile
		bf, err := buildfile.LoadFromFile()
		if err != nil {
			fmt.Printf("Error loading Buildfile.yaml: %v\n", err)
			return
		}
		bf.EnsureInit()

		// Resolve which actions to perform
		syncRepos := doSyncHelmRepos || doAll
		syncCharts := doEnsureCharts || doAll
		applyManifests := doApplyManifests || doAll
		runStartup := doRunStartup || doAll

		if !syncRepos && !syncCharts && !applyManifests && !runStartup {
			fmt.Println("No actions specified. Use flags or --all to perform all actions.")
			return
		}

		if syncRepos {
			if err := helmSyncRepos(bf); err != nil {
				fmt.Printf("Error syncing Helm repositories: %v\n", err)
				return
			}
		}

		if syncCharts {
			if err := helmEnsureCharts(bf); err != nil {
				fmt.Printf("Error ensuring Helm charts: %v\n", err)
				return
			}
		}

		if applyManifests {
			if err := kubectlApplyAll(bf); err != nil {
				fmt.Printf("Error applying Kubernetes manifests: %v\n", err)
				return
			}
		}

		if runStartup {
			if err := runStartupScripts(bf); err != nil {
				fmt.Printf("Error running startup scripts: %v\n", err)
				return
			}
		}

		fmt.Println("syncToCluster completed.")
	},
}

func init() {
	rootCmd.AddCommand(syncToClusterCmd)

	syncToClusterCmd.Flags().BoolVar(&doSyncHelmRepos, "sync-helm-repos", false, "Sync local Helm repos to match Buildfile's helm_repos")
	syncToClusterCmd.Flags().BoolVar(&doEnsureCharts, "ensure-charts", false, "Ensure Helm charts from Buildfile are installed in the cluster")
	syncToClusterCmd.Flags().BoolVar(&doApplyManifests, "apply-manifests", false, "Run 'kubectl apply -f' for each item in k8s_manifests")
	syncToClusterCmd.Flags().BoolVar(&doRunStartup, "run-startup", false, "Run all scripts listed in startup_scripts using /bin/sh -c")
	syncToClusterCmd.Flags().BoolVar(&doAll, "all", false, "Perform all actions: repos, charts, manifests, and startup scripts")
}

func helmSyncRepos(bf *buildfile.SetupFile) error {
	settings := buildfile.InitHelmClient()
	f, err := repo.LoadFile(settings.RepositoryConfig)
	if err != nil {
		return err
	}

	addedOrUpdated := 0
	for name, url := range bf.HelmRepos {
		// Check if exists and matches URL
		var existing *repo.Entry
		for _, r := range f.Repositories {
			if strings.EqualFold(r.Name, name) {
				existing = r
				break
			}
		}

		if existing != nil {
			if trimURL(existing.URL) == trimURL(url) {
				continue // up-to-date
			}
		}

		entry := &repo.Entry{Name: name, URL: url}
		cr, err := repo.NewChartRepository(entry, getter.All(settings))
		if err != nil {
			return err
		}
		if _, err = cr.DownloadIndexFile(); err != nil {
			return fmt.Errorf("failed to download index for repo %s: %w", name, err)
		}

		if existing == nil {
			f.Update(entry) // adds if not existing
		} else {
			// Replace URL
			existing.URL = url
		}
		addedOrUpdated++
	}

	if addedOrUpdated > 0 {
		if err := f.WriteFile(settings.RepositoryConfig, 0644); err != nil {
			return err
		}
		fmt.Printf("Helm repositories added/updated: %d\n", addedOrUpdated)
	} else {
		fmt.Println("Helm repositories already in sync with Buildfile.")
	}
	return nil
}

func helmEnsureCharts(bf *buildfile.SetupFile) error {
	settings := cli.New()
	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), "", "secrets", func(string, ...interface{}) {}); err != nil {
		return err
	}

	installed := 0
	for release, chartRef := range bf.HelmCharts {
		u := action.NewUpgrade(cfg)
		u.Install = true
		u.Namespace = settings.Namespace()
		// Use Upgrade's ChartPathOptions to resolve chart reference
		chartPath, err := u.ChartPathOptions.LocateChart(chartRef, settings)
		if err != nil {
			return fmt.Errorf("failed to locate chart for release %s (%s): %w", release, chartRef, err)
		}

		ch, err := loader.Load(chartPath)
		if err != nil {
			return fmt.Errorf("failed to load chart for release %s: %w", release, err)
		}

		vals := map[string]interface{}{}
		if _, err := u.Run(release, ch, vals); err != nil {
			return fmt.Errorf("failed to install/upgrade release %s: %w", release, err)
		}
		installed++
	}

	if installed > 0 {
		fmt.Printf("Ensured %d Helm release(s).\n", installed)
	} else {
		fmt.Println("No Helm releases defined in Buildfile or all are up to date.")
	}
	return nil
}

func kubectlApplyAll(bf *buildfile.SetupFile) error {
	if len(bf.Manifests) == 0 {
		fmt.Println("No k8s_manifests defined in Buildfile.")
		return nil
	}
	applied := 0
	for _, path := range bf.Manifests {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		cmd := exec.Command("kubectl", "apply", "-f", path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("kubectl apply -f %s failed: %w", path, err)
		}
		applied++
	}
	fmt.Printf("Applied %d manifest file(s).\n", applied)
	return nil
}

func runStartupScripts(bf *buildfile.SetupFile) error {
	if len(bf.StartupScripts) == 0 {
		fmt.Println("No startup_scripts defined in Buildfile.")
		return nil
	}
	run := 0
	for _, script := range bf.StartupScripts {
		script = strings.TrimSpace(script)
		if script == "" {
			continue
		}
		// Execute via sh -c to allow non-executable scripts
		cmd := exec.Command("/bin/sh", "-c", script)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed running startup script '%s': %w", script, err)
		}
		run++
	}
	fmt.Printf("Ran %d startup script(s).\n", run)
	return nil
}

func trimURL(u string) string { return strings.TrimRight(strings.ToLower(u), "/") }

// simple helper to combine multiple errors (not heavily used here, but kept for future extension)
type multiError struct{ errs []error }

func (m *multiError) Error() string {
	if len(m.errs) == 0 { return "" }
	var b strings.Builder
	for i, e := range m.errs {
		if i > 0 { b.WriteString("; ") }
		b.WriteString(e.Error())
	}
	return b.String()
}
func (m *multiError) Append(err error) { if err != nil { m.errs = append(m.errs, err) } }
func (m *multiError) OrNil() error { if len(m.errs) == 0 { return nil }; return m }

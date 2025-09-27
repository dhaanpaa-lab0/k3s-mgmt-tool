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

	"github.com/spf13/cobra"
	"nexus-sites.net/k3s-mgmt-tool/pkg/buildfile"
)

var addMissingCharts bool
var addMissingRepos bool
var addAll bool

// syncFromClusterCmd represents the syncFromCluster command
var syncFromClusterCmd = &cobra.Command{
	Use:   "syncFromCluster",
	Short: "Sync Buildfile from local Helm and/or cluster based on flags",
	Long:  `Optionally update the Buildfile based on the current environment. Use --add-missing-repos to add Helm repos found locally but missing in the Buildfile. Use --add-missing-charts to add charts installed in the cluster but missing in the Buildfile. Use --all to perform both actions.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load Buildfile
		t, err := buildfile.LoadFromFile()
		if err != nil {
			fmt.Printf("Error loading Buildfile: %v\n", err)
			return
		}

		doRepos := addMissingRepos || addAll
		doCharts := addMissingCharts || addAll

		if !doRepos && !doCharts {
			fmt.Println("No actions specified. Use --add-missing-repos and/or --add-missing-charts, or --all to perform both.")
			return
		}

		reposAdded := 0
		if doRepos {
			// Compute differences against local Helm config
			diff, derr := t.DiffHelmReposWithLocal()
			if derr != nil {
				fmt.Printf("Error reading local Helm repositories: %v\n", derr)
				return
			}
			for name, url := range diff.MissingInSetup {
				// add repos present locally but missing in Buildfile
				t.AddHelmRepo(name, url)
				reposAdded++
			}
		}

		chartsAdded := 0
		if doCharts {
			missingCharts, cerr := t.DiffInstalledChartsMissingInBuildfile()
			if cerr != nil {
				fmt.Printf("Error listing installed Helm charts: %v\n", cerr)
			} else {
				for rel, chart := range missingCharts {
					if t.AddHelmChart(rel, chart) {
						chartsAdded++
					}
				}
			}
		}

		if reposAdded == 0 && chartsAdded == 0 {
			if doRepos && doCharts {
				fmt.Println("No repositories or releases to add; Buildfile already includes all local Helm repos and installed release names.")
			} else if doRepos {
				fmt.Println("No repositories to add; Buildfile already includes all local Helm repos.")
			} else if doCharts {
				fmt.Println("No releases to add; Buildfile already includes all installed release names.")
			}
			return
		}

		if err := t.SaveToFile(); err != nil {
			fmt.Printf("Error saving Buildfile with added items: %v\n", err)
			return
		}

		if reposAdded > 0 {
			fmt.Printf("Added %d repository(ies) to Buildfile.\n", reposAdded)
		}
		if chartsAdded > 0 {
			fmt.Printf("Added %d release(s) to Buildfile.\n", chartsAdded)
		}
		fmt.Println("Saved changes to Buildfile.")
	},
}

func init() {
	rootCmd.AddCommand(syncFromClusterCmd)
	// Flags to optionally add missing repos/charts from environment to Buildfile
	syncFromClusterCmd.Flags().BoolVar(&addMissingRepos, "add-missing-repos", false, "Add Helm repos present locally but missing in Buildfile")
	syncFromClusterCmd.Flags().BoolVar(&addMissingCharts, "add-missing-charts", false, "Add charts installed in cluster but missing in Buildfile")
	syncFromClusterCmd.Flags().BoolVar(&addAll, "all", false, "Perform both --add-missing-repos and --add-missing-charts")
}

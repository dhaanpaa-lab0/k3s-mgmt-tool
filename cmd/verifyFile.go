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

// verifyFileCmd represents the verifyFile command
var verifyFileCmd = &cobra.Command{
	Use:   "verifyFile",
	Short: "Display K3D Setup File Information and compare with local Helm",
	Long:  `Display the K3D cluster setup file and compare its Helm repositories with the local Helm configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		t, err := buildfile.LoadFromFile()
		if err == nil {
			fmt.Println("Helm Repositories defined ....")
			for key, repo := range t.HelmRepos {
				fmt.Printf("Repository: %s, URL: %s\n", key, repo)
			}
			fmt.Println()
			fmt.Println("Helm Charts defined ....")
			for release, chart := range t.HelmCharts {
				fmt.Printf("Release: %s -> Chart: %s\n", release, chart)
			}
			fmt.Println()
			fmt.Println("K8s Manifests defined ....")
			for _, m := range t.Manifests {
				fmt.Printf("Manifest: %s\n", m)
			}
			fmt.Println()
			fmt.Println("Startup Scripts defined ....")
			for _, s := range t.StartupScripts {
				fmt.Printf("Script: %s\n", s)
			}

			// Compute and display differences with local Helm configuration
			diff, derr := t.DiffHelmReposWithLocal()
			if derr != nil {
				fmt.Printf("\nError reading local Helm repositories: %v\n", derr)
				return
			}
			fmt.Println()
			fmt.Println("Comparison with local Helm repositories ....")
			if len(diff.MissingInLocal) == 0 && len(diff.MissingInSetup) == 0 && len(diff.URLMismatches) == 0 {
				fmt.Println("No differences found. Buildfile and local Helm repositories are in sync.")
			} else {
				if len(diff.MissingInLocal) > 0 {
					fmt.Println("- Present in Buildfile but missing locally:")
					for name, url := range diff.MissingInLocal {
						fmt.Printf("  %s -> %s\n", name, url)
					}
				}
				if len(diff.MissingInSetup) > 0 {
					fmt.Println("- Present locally but missing in Buildfile:")
					for name, url := range diff.MissingInSetup {
						fmt.Printf("  %s -> %s\n", name, url)
					}
				}
				if len(diff.URLMismatches) > 0 {
					fmt.Println("- Name matches but URLs differ:")
					for name, pair := range diff.URLMismatches {
						fmt.Printf("  %s: Buildfile=%s, Local=%s\n", name, pair.SetupURL, pair.LocalURL)
					}
				}
			}


			// Check for installed Helm releases in cluster missing from Buildfile
			chartsMissing, cerr := t.DiffInstalledChartsMissingInBuildfile()
			if cerr != nil {
				fmt.Printf("\nError listing installed Helm charts: %v\n", cerr)
			} else {
				fmt.Println()
				fmt.Println("Comparison with cluster installed Helm releases ....")
				if len(chartsMissing) == 0 {
					fmt.Println("No missing releases. Buildfile contains all installed release names.")
				} else {
					fmt.Println("- Releases installed in cluster but missing in Buildfile (release -> chart):")
					for rel, chart := range chartsMissing {
						fmt.Printf("  %s -> %s\n", rel, chart)
					}
				}
			}
		} else {
			fmt.Printf("Error loading K3D Setup File: %s\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(verifyFileCmd)
}

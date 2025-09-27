package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"nexus-sites.net/k3s-mgmt-tool/pkg/buildfile"
)

var (
	flagForce      bool
	flagDryRun     bool
	flagHelmBinary string
)

// repoSetupCmd creates _reposetup/[helm_chart_name]/values.yaml for each chart in Buildfile.yaml
var repoSetupCmd = &cobra.Command{
	Use:   "reposetup",
	Short: "Generate _reposetup/<chart>/values.yaml for all charts in Buildfile.yaml",
	Long:  "Creates a folder structure _reposetup/<helm_chart_name> and generates default values.yaml files for each chart defined in Buildfile.yaml.",
	RunE: func(cmd *cobra.Command, args []string) error {
		bf, err := buildfile.LoadFromFile()
		if err != nil {
			return fmt.Errorf("failed to load Buildfile.yaml: %w", err)
		}
		bf.EnsureInit()

		if len(bf.HelmCharts) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No Helm charts defined in Buildfile.yaml")
			return nil
		}

		if verbose {
			fmt.Fprintln(cmd.OutOrStdout(), "Charts in Buildfile:")
			for rel, chart := range bf.HelmCharts {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s -> %s\n", rel, chart)
			}
		}

		root := "_reposetup"
		if err := os.MkdirAll(root, 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", root, err)
		}

		var firstErr error
		for _, chart := range bf.HelmCharts {
			chart = strings.TrimSpace(chart)
			if chart == "" {
				continue
			}
			// Use chart path as folder name. If it contains '/', this will create nested folders which is OK
			dir := filepath.Join(root, chart)
			valuesPath := filepath.Join(dir, "values.yaml")

			if flagDryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would create dir: %s\n", dir)
				if _, err := os.Stat(valuesPath); err == nil && !flagForce {
					fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] values.yaml exists, would skip (use --force to overwrite): %s\n", valuesPath)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would write values.yaml for chart '%s' to %s\n", chart, valuesPath)
				}
				continue
			}

			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "Creating directory (if not exists): %s\n", dir)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("failed creating dir %s: %w", dir, err)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), firstErr)
				continue
			}

			if _, err := os.Stat(valuesPath); err == nil && !flagForce {
				if verbose {
					fmt.Fprintf(cmd.OutOrStdout(), "values.yaml exists and --force not set, skipping: %s\n", valuesPath)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Skipping existing values.yaml (use --force to overwrite): %s\n", valuesPath)
				continue
			}

			// Try to fetch default values. If chart is unqualified (no '/'), also try repo-qualified candidates using helm_repos from Buildfile.
			var content []byte
			var fetchErr error
			candidates := []string{}
			baseChart := chart
			if !strings.Contains(baseChart, "/") {
				// Prefer repo with the same name as the chart if present, e.g., traefik -> traefik/traefik
				if _, ok := bf.HelmRepos[baseChart]; ok {
					candidates = append(candidates, baseChart+"/"+baseChart)
				}
				// Try every defined repo as a prefix
				for repoName := range bf.HelmRepos {
					if repoName == baseChart {
						continue // already added
					}
					candidates = append(candidates, repoName+"/"+baseChart)
				}
			}
			// Finally, try the chart as-is
			candidates = append(candidates, baseChart)

			for i, ref := range candidates {
				if verbose {
					if i == 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "Running: %s show values %s\n", flagHelmBinary, ref)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "Retrying: %s show values %s\n", flagHelmBinary, ref)
					}
				}
				var try []byte
				try, fetchErr = helmShowValues(flagHelmBinary, ref)
				if fetchErr == nil {
					content = try
					// If we used a different ref than originally specified, let the user know in verbose mode
					if verbose && ref != baseChart {
						fmt.Fprintf(cmd.OutOrStdout(), "Resolved chart '%s' via '%s'\n", baseChart, ref)
					}
					break
				}
				if verbose {
					fmt.Fprintf(cmd.ErrOrStderr(), "helm show values error for %s: %v\n", ref, fetchErr)
				}
			}

			if content == nil {
				// Fallback to placeholder if helm is not available or chart can't be resolved
				placeholder := fmt.Sprintf("# Unable to retrieve default values for chart '%s' using '%s'.\n# Ensure helm is installed and repositories are added (e.g., 'helm repo add').\n# You can manually populate this file.\n", chart, flagHelmBinary)
				content = []byte(placeholder)
			}

			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "Writing values.yaml: %s\n", valuesPath)
			}
			if err := os.WriteFile(valuesPath, content, 0o644); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("failed writing %s: %w", valuesPath, err)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), firstErr)
				continue
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", valuesPath)
		}

		return firstErr
	},
}

func init() {
	rootCmd.AddCommand(repoSetupCmd)

	repoSetupCmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite existing values.yaml if present")
	repoSetupCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Preview actions without writing files")
	repoSetupCmd.Flags().StringVar(&flagHelmBinary, "helm-binary", "helm", "Path to helm binary to use for fetching default values")
}

func helmShowValues(helmBin, chart string) ([]byte, error) {
	if strings.TrimSpace(helmBin) == "" {
		helmBin = "helm"
	}
	cmd := exec.Command(helmBin, "show", "values", chart)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ee := (&exec.Error{}); errors.As(err, &ee) {
			return nil, fmt.Errorf("helm not found: %w", err)
		}
		return nil, fmt.Errorf("helm show values failed for %s: %v\n%s", chart, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"nexus-sites.net/k3s-mgmt-tool/pkg/buildfile"
)

func addCommon(cmd *cobra.Command, msg string, changed bool, saveErr error) {
	if saveErr != nil {
		fmt.Printf("Error saving K3D Setup File: %s\n", saveErr)
		return
	}
	if changed {
		fmt.Println(msg)
	} else {
		fmt.Println("No changes were made (item may already exist).")
	}
}

// addRepoCmd adds or updates a Helm repo entry
var addRepoCmd = &cobra.Command{
	Use:   "addRepo <name> <url>",
	Short: "Add or update a Helm repository",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := buildfile.LoadFromFile()
		if err != nil {
			fmt.Printf("Error loading K3D Setup File: %s\n", err)
			return
		}
		f.AddHelmRepo(args[0], args[1])
		if err := f.SaveToFile(); err != nil {
			fmt.Printf("Error saving K3D Setup File: %s\n", err)
			return
		}
		fmt.Printf("Repo '%s' set to '%s'.\n", args[0], args[1])
	},
}

// removeRepoCmd removes a Helm repo by name
var removeRepoCmd = &cobra.Command{
	Use:   "removeRepo <name>",
	Short: "Remove a Helm repository",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := buildfile.LoadFromFile()
		if err != nil {
			fmt.Printf("Error loading K3D Setup File: %s\n", err)
			return
		}
		if f.RemoveHelmRepo(args[0]) {
			if err := f.SaveToFile(); err != nil {
				fmt.Printf("Error saving K3D Setup File: %s\n", err)
				return
			}
			fmt.Printf("Repo '%s' removed.\n", args[0])
		} else {
			fmt.Println("Repository not found; no changes made.")
		}
	},
}

// addChartCmd adds a Helm chart entry mapped by release name
var addChartCmd = &cobra.Command{
	Use:   "addChart <release> <chart>",
	Short: "Add a Helm chart mapping (release -> chart)",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := buildfile.LoadFromFile()
		if err != nil {
			fmt.Printf("Error loading K3D Setup File: %s\n", err)
			return
		}
		changed := f.AddHelmChart(args[0], args[1])
		addCommon(cmd, "Chart mapping added.", changed, f.SaveToFile())
	},
}

// removeChartCmd removes a Helm chart entry by release name
var removeChartCmd = &cobra.Command{
	Use:   "removeChart <release>",
	Short: "Remove a Helm chart mapping by release name",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := buildfile.LoadFromFile()
		if err != nil {
			fmt.Printf("Error loading K3D Setup File: %s\n", err)
			return
		}
		if f.RemoveHelmChart(args[0]) {
			if err := f.SaveToFile(); err != nil {
				fmt.Printf("Error saving K3D Setup File: %s\n", err)
				return
			}
			fmt.Println("Chart mapping removed.")
		} else {
			fmt.Println("Release not found; no changes made.")
		}
	},
}

// addManifestCmd adds a manifest path
var addManifestCmd = &cobra.Command{
	Use:   "addManifest <path>",
	Short: "Add a Kubernetes manifest path",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := buildfile.LoadFromFile()
		if err != nil {
			fmt.Printf("Error loading K3D Setup File: %s\n", err)
			return
		}
		changed := f.AddManifest(args[0])
		addCommon(cmd, "Manifest added.", changed, f.SaveToFile())
	},
}

// removeManifestCmd removes a manifest path
var removeManifestCmd = &cobra.Command{
	Use:   "removeManifest <path>",
	Short: "Remove a Kubernetes manifest path",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := buildfile.LoadFromFile()
		if err != nil {
			fmt.Printf("Error loading K3D Setup File: %s\n", err)
			return
		}
		if f.RemoveManifest(args[0]) {
			if err := f.SaveToFile(); err != nil {
				fmt.Printf("Error saving K3D Setup File: %s\n", err)
				return
			}
			fmt.Println("Manifest removed.")
		} else {
			fmt.Println("Manifest not found; no changes made.")
		}
	},
}

// addScriptCmd adds a startup script path
var addScriptCmd = &cobra.Command{
	Use:   "addScript <path>",
	Short: "Add a startup script path",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := buildfile.LoadFromFile()
		if err != nil {
			fmt.Printf("Error loading K3D Setup File: %s\n", err)
			return
		}
		changed := f.AddStartupScript(args[0])
		addCommon(cmd, "Startup script added.", changed, f.SaveToFile())
	},
}

// removeScriptCmd removes a startup script path
var removeScriptCmd = &cobra.Command{
	Use:   "removeScript <path>",
	Short: "Remove a startup script path",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := buildfile.LoadFromFile()
		if err != nil {
			fmt.Printf("Error loading K3D Setup File: %s\n", err)
			return
		}
		if f.RemoveStartupScript(args[0]) {
			if err := f.SaveToFile(); err != nil {
				fmt.Printf("Error saving K3D Setup File: %s\n", err)
				return
			}
			fmt.Println("Startup script removed.")
		} else {
			fmt.Println("Startup script not found; no changes made.")
		}
	},
}

func init() {
	rootCmd.AddCommand(addRepoCmd)
	rootCmd.AddCommand(removeRepoCmd)
	rootCmd.AddCommand(addChartCmd)
	rootCmd.AddCommand(removeChartCmd)
	rootCmd.AddCommand(addManifestCmd)
	rootCmd.AddCommand(removeManifestCmd)
	rootCmd.AddCommand(addScriptCmd)
	rootCmd.AddCommand(removeScriptCmd)
}

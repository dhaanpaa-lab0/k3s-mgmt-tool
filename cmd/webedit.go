package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
	"nexus-sites.net/k3s-mgmt-tool/pkg/web"
)

// webEditCmd starts a local web editor that serves an embedded UI
var webEditCmd = &cobra.Command{
	Use:   "webedit",
	Short: "Open a local web-based editor for Buildfile.yaml (Material UI + htmx)",
	RunE: func(cmd *cobra.Command, args []string) error {
		server := &web.Server{Addr: "127.0.0.1:0"}
		url, err := server.Start()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Web editor listening at %s\n", url)
		_ = openBrowser(url)
		return server.WaitForInterrupt()
	},
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return nil
	}
}

func init() {
	rootCmd.AddCommand(webEditCmd)
}

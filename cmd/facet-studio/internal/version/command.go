package version

import (
	"github.com/spf13/cobra"

	"github.com/xibodev/facet-studio/cmd/facet-studio/internal"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/cliui"
	"github.com/xibodev/facet-studio/pkg/config"
)

func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "Show version information",
		Run: func(_ *cobra.Command, _ []string) {
			printVersion()
		},
	}

	return cmd
}

func printVersion() {
	build, goVer := config.FormatBuildInfo()
	cliui.PrintVersion(internal.Logo, "facet-studio "+config.FormatVersion(), build, goVer)
}

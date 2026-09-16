// Facet Studio - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Facet Studio contributors

package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xibodev/facet-studio/cmd/facet-studio/internal"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/agent"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/auth"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/cliui"
	configcmd "github.com/xibodev/facet-studio/cmd/facet-studio/internal/config"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/cron"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/gateway"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/mcp"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/migrate"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/model"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/onboard"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/skills"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/status"
	"github.com/xibodev/facet-studio/cmd/facet-studio/internal/version"
	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet-studio/pkg/updater"
)

var rootNoColor bool

// initTermuxSSL detects Termux environment and sets SSL_CERT_FILE if not already set.
// This fixes X509 certificate errors when running Facet Studio inside Termux or termux-chroot.
// See: https://github.com/xibodev/facet-studio/issues/2944
func initTermuxSSL() {
	// Only applicable on Linux/Android
	if runtime.GOOS != "linux" && runtime.GOOS != "android" {
		return
	}

	// Skip if already set
	if os.Getenv("SSL_CERT_FILE") != "" {
		return
	}

	// Check for Termux prefix in PATH or HOME
	home := os.Getenv("HOME")
	path := os.Getenv("PATH")

	isTermux := strings.Contains(home, "com.termux") ||
		strings.Contains(path, "com.termux") ||
		strings.Contains(home, "/data/data/com.termux")

	if !isTermux {
		return
	}

	// Check common CA bundle locations in Termux
	caPaths := []string{
		"$PREFIX/etc/tls/cert.pem",
		os.Getenv("PREFIX") + "/etc/tls/cert.pem",
		"/data/data/com.termux/files/usr/etc/tls/cert.pem",
		"/usr/etc/tls/cert.pem",
	}

	for _, caPath := range caPaths {
		expanded := os.ExpandEnv(caPath)
		if _, err := os.Stat(expanded); err == nil {
			os.Setenv("SSL_CERT_FILE", expanded)
			return
		}
	}
}

func syncCliUIColor(root *cobra.Command) {
	no, _ := root.PersistentFlags().GetBool("no-color")
	cliui.Init(no || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb")
}

// earlyColorDisabled matches lipgloss/banner behavior from env and argv before Cobra parses flags.
func earlyColorDisabled() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return true
	}
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--no-color" || arg == "--no-color=true" || arg == "--no-color=1" {
			return true
		}
	}
	return false
}

func NewFacetStudioCommand() *cobra.Command {
	short := fmt.Sprintf("%s Facet Studio — agent host with detached modules", internal.Logo)
	long := fmt.Sprintf(`%s Facet Studio is an agent host and web cockpit.

Capabilities come from detached local modules, discovered and invoked as
bounded separate processes rather than linked into this binary.

Version: %s`, internal.Logo, config.FormatVersion())

	cmd := &cobra.Command{
		Use:   "facet-studio",
		Short: short,
		Long:  long,
		Example: `facet-studio version
facet-studio modules
facet-studio gateway`,
		SilenceErrors: true,
		// Avoid plain UsageString() on stderr/stdout when a command fails; cliui
		// renders matching panels on stderr instead.
		SilenceUsage: true,
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			syncCliUIColor(c.Root())
		},
	}

	cmd.PersistentFlags().BoolVar(&rootNoColor, "no-color", false,
		"Disable colors (boxed layout unchanged)")

	cmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		syncCliUIColor(c.Root())
		fmt.Fprint(c.OutOrStdout(), cliui.RenderCommandHelp(c))
	})

	cmd.AddCommand(
		configcmd.NewConfigCommand(),
		onboard.NewOnboardCommand(),
		agent.NewAgentCommand(),
		auth.NewAuthCommand(),
		gateway.NewGatewayCommand(),
		status.NewStatusCommand(),
		cron.NewCronCommand(),
		mcp.NewMCPCommand(),
		migrate.NewMigrateCommand(),
		skills.NewSkillsCommand(),
		model.NewModelCommand(),
		updater.NewUpdateCommand("facet-studio"),
		version.NewVersionCommand(),
		NewModulesCommand(),
		NewModuleInvokeCommand(),
		NewHandoffCommand(),
		NewModuleAddCommand(),
		NewModuleEnableCommand(),
		NewModuleDisableCommand(),
		NewModuleRemoveCommand(),
	)

	return cmd
}

const (
	colorBlue = "\033[1;38;2;62;93;185m"
	colorRed  = "\033[1;38;2;213;70;70m"
	banner    = "\r\n" +
		colorBlue + "███████╗ █████╗  ██████╗███████╗████████╗" + colorRed + " ███████╗████████╗██╗   ██╗██████╗ ██╗ ██████╗ \n" +
		colorBlue + "██╔════╝██╔══██╗██╔════╝██╔════╝╚══██╔══╝" + colorRed + " ██╔════╝╚══██╔══╝██║   ██║██╔══██╗██║██╔═══██╗\n" +
		colorBlue + "█████╗  ███████║██║     █████╗     ██║   " + colorRed + " ███████╗   ██║   ██║   ██║██║  ██║██║██║   ██║\n" +
		colorBlue + "██╔══╝  ██╔══██║██║     ██╔══╝     ██║   " + colorRed + " ╚════██║   ██║   ██║   ██║██║  ██║██║██║   ██║\n" +
		colorBlue + "██║     ██║  ██║╚██████╗███████╗   ██║   " + colorRed + " ███████║   ██║   ╚██████╔╝██████╔╝██║╚██████╔╝\n" +
		colorBlue + "╚═╝     ╚═╝  ╚═╝ ╚═════╝╚══════╝   ╚═╝   " + colorRed + " ╚══════╝   ╚═╝    ╚═════╝ ╚═════╝ ╚═╝ ╚═════╝ \n " +
		"\033[0m\r\n"
	plainBanner = "\r\n" +
		"███████╗ █████╗  ██████╗███████╗████████╗ ███████╗████████╗██╗   ██╗██████╗ ██╗ ██████╗ \n" +
		"██╔════╝██╔══██╗██╔════╝██╔════╝╚══██╔══╝ ██╔════╝╚══██╔══╝██║   ██║██╔══██╗██║██╔═══██╗\n" +
		"█████╗  ███████║██║     █████╗     ██║    ███████╗   ██║   ██║   ██║██║  ██║██║██║   ██║\n" +
		"██╔══╝  ██╔══██║██║     ██╔══╝     ██║    ╚════██║   ██║   ██║   ██║██║  ██║██║██║   ██║\n" +
		"██║     ██║  ██║╚██████╗███████╗   ██║    ███████║   ██║   ╚██████╔╝██████╔╝██║╚██████╔╝\n" +
		"╚═╝     ╚═╝  ╚═╝ ╚═════╝╚══════╝   ╚═╝    ╚══════╝   ╚═╝    ╚═════╝ ╚═════╝ ╚═╝ ╚═════╝ \n " +
		"\r\n"
)

func main() {
	// Initialize Termux SSL certificate detection before anything else
	initTermuxSSL()

	cliui.Init(earlyColorDisabled())

	if earlyColorDisabled() {
		fmt.Print(plainBanner)
	} else {
		fmt.Printf("%s", banner)
	}

	tzEnv := os.Getenv("TZ")
	if tzEnv != "" {
		fmt.Println("TZ environment:", tzEnv)
		zoneinfoEnv := os.Getenv("ZONEINFO")
		fmt.Println("ZONEINFO environment:", zoneinfoEnv)
		loc, err := time.LoadLocation(tzEnv)
		if err != nil {
			fmt.Println("Error loading time zone:", err)
		} else {
			fmt.Println("Time zone loaded successfully:", loc)
			time.Local = loc //nolint:gosmopolitan // We intentionally set local timezone from TZ env
		}
	}

	cmd := NewFacetStudioCommand()
	last, err := cmd.ExecuteC()
	if err != nil {
		syncCliUIColor(cmd)
		fmt.Fprint(os.Stderr, cliui.FormatCLIError(err.Error(), last))
		os.Exit(1)
	}
}

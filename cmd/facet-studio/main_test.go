package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xibodev/facet-studio/cmd/facet-studio/internal"
	"github.com/xibodev/facet-studio/pkg/config"
)

func TestNewFacetStudioCommand(t *testing.T) {
	cmd := NewFacetStudioCommand()

	require.NotNil(t, cmd)

	short := fmt.Sprintf("%s Facet Studio — agent host with detached modules", internal.Logo)
	longHas := strings.Contains(cmd.Long, config.FormatVersion())

	assert.Equal(t, "facet-studio", cmd.Use)
	assert.Equal(t, short, cmd.Short)
	assert.True(t, longHas)

	assert.True(t, cmd.HasSubCommands())
	assert.True(t, cmd.HasAvailableSubCommands())

	assert.True(t, cmd.PersistentFlags().Lookup("no-color") != nil)

	assert.Nil(t, cmd.Run)
	assert.Nil(t, cmd.RunE)

	assert.NotNil(t, cmd.PersistentPreRun)
	assert.Nil(t, cmd.PersistentPostRun)

	allowedCommands := []string{
		"agent",
		"auth",
		"config",
		"cron",
		"gateway",
		"handoff",
		"mcp",
		"migrate",
		"model",
		// The module commands. This list is an ALLOWLIST -- a new subcommand
		// is meant to fail here until someone adds it deliberately, which is
		// exactly what it did. It had simply been left red.
		"module-invoke",
		"modules",
		"modules-add",
		"modules-disable",
		"modules-enable",
		"modules-remove",
		"onboard",
		"skills",
		"status",
		"update",
		"version",
	}

	subcommands := cmd.Commands()
	assert.Len(t, subcommands, len(allowedCommands))

	for _, subcmd := range subcommands {
		found := slices.Contains(allowedCommands, subcmd.Name())
		assert.True(t, found, "unexpected subcommand %q", subcmd.Name())

		assert.False(t, subcmd.Hidden)
	}
}

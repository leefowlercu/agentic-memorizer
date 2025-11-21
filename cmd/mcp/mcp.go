package mcp

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/cmd/mcp/subcommands"
	"github.com/spf13/cobra"
)

var McpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP (Model Context Protocol) server commands",
	Long: "\nManage the MCP (Model Context Protocol) server for AI tool integration.\n\n" +
		"The MCP server provides a standardized interface for AI tools like GitHub Copilot CLI " +
		"and Claude Code to access your semantic memory index through resources, tools, and prompts.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("please specify a subcommand: start")
	},
}

func init() {
	McpCmd.AddCommand(subcommands.StartCmd)
	// Future: test, validate, status, config, debug, trace commands
}

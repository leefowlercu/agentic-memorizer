package opencodecli

import (
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
)

const (
	pluginDir = "~/.config/opencode/plugins/memorizer"
)

// pluginLuaContent is the Lua plugin for OpenCode.
var pluginLuaContent = []byte(`-- Memorizer plugin for OpenCode
-- Provides knowledge graph access within OpenCode sessions

local M = {}

-- Plugin metadata
M.name = "memorizer"
M.version = "1.0.0"
M.description = "Knowledge graph integration for Agentic Memorizer"

-- Initialize the plugin
function M.setup(opts)
    opts = opts or {}
    M.format = opts.format or "toon"
    M.quiet = opts.quiet ~= false
end

-- Read the knowledge graph
function M.read(format)
    format = format or M.format
    local cmd = "memorizer read --format " .. format
    if M.quiet then
        cmd = cmd .. " --quiet"
    end

    local handle = io.popen(cmd)
    if handle then
        local result = handle:read("*a")
        handle:close()
        return result
    end
    return nil, "Failed to execute memorizer command"
end

-- Get knowledge graph status
function M.status()
    local handle = io.popen("memorizer daemon status 2>&1")
    if handle then
        local result = handle:read("*a")
        handle:close()
        return result
    end
    return nil, "Failed to get daemon status"
end

return M
`)

// NewPluginIntegration creates an OpenCode plugin integration.
// This integration installs the memorizer Lua plugin for OpenCode.
func NewPluginIntegration() integrations.Integration {
	return integrations.NewPluginIntegration(
		"opencode-plugin",
		harnessName,
		"OpenCode plugin integration that provides knowledge graph access via Lua API",
		binaryName,
		pluginDir,
		[]integrations.PluginFile{
			{
				TargetName: "init.lua",
				Content:    pluginLuaContent,
				Mode:       0644,
			},
		},
	)
}

func init() {
	_ = integrations.RegisterIntegration(NewPluginIntegration())
}

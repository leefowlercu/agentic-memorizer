package protocol

// ResourceListChangedNotification is sent when the list of available resources changes
// This is a server-initiated notification sent to subscribed clients
type ResourceListChangedNotification struct {
	// Empty params - clients should call resources/list to get updated list
}

// PromptListChangedNotification is sent when the list of available prompts changes
// This is a server-initiated notification sent to clients
type PromptListChangedNotification struct {
	// Empty params - clients should call prompts/list to get updated list
}

// ToolListChangedNotification is sent when the list of available tools changes
// This is a server-initiated notification sent to clients
type ToolListChangedNotification struct {
	// Empty params - clients should call tools/list to get updated list
}

package hooks

type Settings struct {
	Hooks map[string][]HookEvent `json:"hooks,omitempty"`
}

type HookEvent struct {
	Matcher string `json:"matcher,omitempty"`
	Hooks   []Hook `json:"hooks"`
}

type Hook struct {
	Type    string  `json:"type"`
	Command string  `json:"command"`
	Timeout float64 `json:"timeout,omitempty"`
}

package statusline

// rawInput mirrors the stdin JSON contract from Claude Code. Every optional or
// nullable field is a pointer so parseInput can reproduce jq's `//` defaulting:
// absent and null both decode to nil.
type rawInput struct {
	Model struct {
		DisplayName *string `json:"display_name"`
	} `json:"model"`
	Effort struct {
		Level *string `json:"level"`
	} `json:"effort"`
	Workspace struct {
		CurrentDir  *string `json:"current_dir"`
		GitWorktree *string `json:"git_worktree"`
	} `json:"workspace"`
	ContextWindow struct {
		ContextWindowSize *float64 `json:"context_window_size"`
		UsedPercentage    *float64 `json:"used_percentage"`
		CurrentUsage      *struct {
			InputTokens              *float64 `json:"input_tokens"`
			CacheCreationInputTokens *float64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     *float64 `json:"cache_read_input_tokens"`
			OutputTokens             *float64 `json:"output_tokens"`
		} `json:"current_usage"`
	} `json:"context_window"`
	RateLimits struct {
		FiveHour *rawRateWindow `json:"five_hour"`
		SevenDay *rawRateWindow `json:"seven_day"`
	} `json:"rate_limits"`
	PR *struct {
		Number      *float64 `json:"number"`
		URL         *string  `json:"url"`
		ReviewState *string  `json:"review_state"`
	} `json:"pr"`
	Worktree *struct {
		Name *string `json:"name"`
	} `json:"worktree"`
}

type rawRateWindow struct {
	UsedPercentage *float64 `json:"used_percentage"`
	ResetsAt       *float64 `json:"resets_at"`
}

// Input is the processed, defaulted view the renderers consume. Nil pointers
// mean "absent" for fields that render conditionally (rate limits, PR, resets).
type Input struct {
	ModelName   string
	EffortLevel string
	CWD         string

	ContextSize    int
	UsedPct        float64
	CurInput       int
	CurCacheCreate int
	CurCacheRead   int
	CurOutput      int

	RateFivePct    *float64
	RateSevenPct   *float64
	RateFiveReset  *int64
	RateSevenReset *int64

	PRNumber *int64
	PRUrl    string
	PRState  string

	WorktreeName string
}

package statusline

import "encoding/json"

func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// floatToInt / floatToInt64Ptr convert the leniently-decoded numeric fields.
// Decoding integer-ish JSON as float64 lets a value that arrives as 200000.0
// (rather than 200000) still render, matching jq's tolerance rather than
// blanking the whole line. Truncation matches bash integer arithmetic.
//
// Note: on float-typed input this is deliberately NOT byte-identical to the old
// bash — bash's integer tests errored and fell through to garbage there (and the
// exact result was jq-version-dependent), so there is no sane target to match.
// The parity harness only ever feeds integer JSON, which is the real contract.
func floatToInt(p *float64) int {
	if p == nil {
		return 0
	}
	return int(*p)
}

func floatToInt64Ptr(p *float64) *int64 {
	if p == nil {
		return nil
	}
	v := int64(*p)
	return &v
}

// parseInput decodes the stdin JSON and applies the same defaults the old
// single jq call did: absent/null numeric fields fall back to jq's `//` values,
// and conditionally-rendered fields stay nil to mean "absent".
func parseInput(data []byte) (*Input, error) {
	var raw rawInput
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	in := &Input{
		ModelName:   strDeref(raw.Model.DisplayName),
		EffortLevel: strDeref(raw.Effort.Level),
		CWD:         strDeref(raw.Workspace.CurrentDir),
		ContextSize: 200000,
	}

	if raw.ContextWindow.ContextWindowSize != nil {
		in.ContextSize = int(*raw.ContextWindow.ContextWindowSize)
	}
	if raw.ContextWindow.UsedPercentage != nil {
		in.UsedPct = *raw.ContextWindow.UsedPercentage
	}
	if cu := raw.ContextWindow.CurrentUsage; cu != nil {
		in.CurInput = floatToInt(cu.InputTokens)
		in.CurCacheCreate = floatToInt(cu.CacheCreationInputTokens)
		in.CurCacheRead = floatToInt(cu.CacheReadInputTokens)
		in.CurOutput = floatToInt(cu.OutputTokens)
	}

	if rw := raw.RateLimits.FiveHour; rw != nil {
		in.RateFivePct = rw.UsedPercentage
		in.RateFiveReset = floatToInt64Ptr(rw.ResetsAt)
	}
	if rw := raw.RateLimits.SevenDay; rw != nil {
		in.RateSevenPct = rw.UsedPercentage
		in.RateSevenReset = floatToInt64Ptr(rw.ResetsAt)
	}

	if raw.PR != nil {
		in.PRNumber = floatToInt64Ptr(raw.PR.Number)
		in.PRUrl = strDeref(raw.PR.URL)
		in.PRState = strDeref(raw.PR.ReviewState)
	}

	// worktree.name // workspace.git_worktree // "" — fall back to git_worktree
	// only when worktree.name is absent/null (matching jq's // semantics).
	if raw.Worktree != nil && raw.Worktree.Name != nil {
		in.WorktreeName = *raw.Worktree.Name
	} else {
		in.WorktreeName = strDeref(raw.Workspace.GitWorktree)
	}

	return in, nil
}

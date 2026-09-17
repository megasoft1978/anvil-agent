package main

import (
	"encoding/json"
	"fmt"
)

// experimentOverrides is deliberately a file-level contract instead of a collection of new
// one-off CLI flags. The optimization runner writes one file per attempted configuration, and
// the applied Config is recorded in run_start so a trace is self-describing.
//
// Fields that are nil preserve the normal model profile. In particular, a nil MinP means the
// request omits min_p and exercises the server default; a pointer to 0 means the request sends
// an explicit min_p=0 for E1.
type experimentOverrides struct {
	SchemaVersion         int      `json:"schema_version"`
	ExperimentID          string   `json:"experiment_id,omitempty"`
	Seed                  *int     `json:"seed,omitempty"`
	MinP                  *float64 `json:"min_p,omitempty"`
	MaxTurns              *int     `json:"max_turns,omitempty"`
	PromptProfile         string   `json:"prompt_profile,omitempty"`
	PreserveToolReasoning *bool    `json:"preserve_tool_reasoning,omitempty"`
	ToolSchemaPolicy      string   `json:"tool_schema_policy,omitempty"`
	ForceEditAfterReads   *int     `json:"force_edit_after_reads,omitempty"`
	CloseOutReserve       *bool    `json:"close_out_reserve,omitempty"`
	RetryWithoutEdit      *bool    `json:"retry_without_edit,omitempty"`
	ReadOutputLimit       *int     `json:"read_output_limit,omitempty"`
	SearchOutputLimit     *int     `json:"search_output_limit,omitempty"`
	MaxHistoryBytes       *int     `json:"max_history_bytes,omitempty"`
	TaskReminder          *bool    `json:"task_reminder,omitempty"`
	RichEditFeedback      *bool    `json:"rich_edit_feedback,omitempty"`
	Ledger                *bool    `json:"ledger,omitempty"`
	DedupReads            *bool    `json:"dedup_reads,omitempty"`
	DetectRepeatedEdits   *bool    `json:"detect_repeated_edits,omitempty"`
}

func applyExperimentOverrides(config *Config, raw []byte) error {
	var overrides experimentOverrides
	if err := json.Unmarshal(raw, &overrides); err != nil {
		return fmt.Errorf("invalid experiment config: %w", err)
	}
	if overrides.SchemaVersion != 1 {
		return fmt.Errorf("unsupported experiment config schema_version %d", overrides.SchemaVersion)
	}
	if overrides.ExperimentID != "" {
		config.ExperimentID = overrides.ExperimentID
	}
	if overrides.Seed != nil {
		if *overrides.Seed < 0 {
			return fmt.Errorf("seed must be non-negative")
		}
		config.Seed = overrides.Seed
	}
	if overrides.MinP != nil {
		if *overrides.MinP < 0 || *overrides.MinP > 1 {
			return fmt.Errorf("min_p must be between 0 and 1")
		}
		config.MinP = overrides.MinP
	}
	if overrides.MaxTurns != nil {
		if *overrides.MaxTurns < 1 {
			return fmt.Errorf("max_turns must be positive")
		}
		config.MaxTurns = *overrides.MaxTurns
	}
	if overrides.PromptProfile != "" {
		if overrides.PromptProfile != "baseline" && overrides.PromptProfile != "focused" {
			return fmt.Errorf("unsupported prompt_profile %q", overrides.PromptProfile)
		}
		config.PromptProfile = overrides.PromptProfile
	}
	if overrides.PreserveToolReasoning != nil {
		config.PreserveToolReasoning = *overrides.PreserveToolReasoning
	}
	if overrides.ToolSchemaPolicy != "" {
		if overrides.ToolSchemaPolicy != "dynamic" && overrides.ToolSchemaPolicy != "stable" {
			return fmt.Errorf("unsupported tool_schema_policy %q", overrides.ToolSchemaPolicy)
		}
		config.ToolSchemaPolicy = overrides.ToolSchemaPolicy
	}
	if overrides.ForceEditAfterReads != nil {
		if *overrides.ForceEditAfterReads < 0 {
			return fmt.Errorf("force_edit_after_reads must be non-negative")
		}
		config.ForceEditAfterReads = *overrides.ForceEditAfterReads
	}
	if overrides.CloseOutReserve != nil {
		config.CloseOutReserve = *overrides.CloseOutReserve
	}
	if overrides.RetryWithoutEdit != nil {
		config.RetryWithoutEdit = *overrides.RetryWithoutEdit
	}
	if overrides.ReadOutputLimit != nil {
		if *overrides.ReadOutputLimit < 512 {
			return fmt.Errorf("read_output_limit must be at least 512 bytes")
		}
		config.ReadOutputLimit = *overrides.ReadOutputLimit
	}
	if overrides.SearchOutputLimit != nil {
		if *overrides.SearchOutputLimit < 512 {
			return fmt.Errorf("search_output_limit must be at least 512 bytes")
		}
		config.SearchOutputLimit = *overrides.SearchOutputLimit
	}
	if overrides.MaxHistoryBytes != nil {
		if *overrides.MaxHistoryBytes < 1024 {
			return fmt.Errorf("max_history_bytes must be at least 1024 bytes")
		}
		config.MaxHistoryBytes = *overrides.MaxHistoryBytes
	}
	if overrides.TaskReminder != nil {
		config.TaskReminder = *overrides.TaskReminder
	}
	if overrides.RichEditFeedback != nil {
		config.RichEditFeedback = *overrides.RichEditFeedback
	}
	if overrides.Ledger != nil {
		config.Ledger = *overrides.Ledger
	}
	if overrides.DedupReads != nil {
		config.DedupReads = *overrides.DedupReads
	}
	if overrides.DetectRepeatedEdits != nil {
		config.DetectRepeatedEdits = *overrides.DetectRepeatedEdits
	}
	return nil
}

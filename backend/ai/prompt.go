package ai

import "strings"

// Context limits (Feature 31: "truncate logs to last 200 lines"). maxContextBytes
// is a hard cap so a single huge line can't blow the prompt size.
const (
	maxContextLines = 200
	maxContextBytes = 24 * 1024
)

// Task identifiers select a system-prompt preamble. An unknown/empty task uses
// the general assistant prompt.
const (
	TaskExplainLog     = "explain_log"
	TaskDiagnose       = "diagnose"
	TaskExplainConfig  = "explain_config"
	TaskSuggestFix     = "suggest_fix"
)

const basePreamble = "You are Stratum's built-in assistant, helping a homelab/small-team operator " +
	"manage Linux hosts, Docker containers, and files. Be concise and concrete. " +
	"When you propose a shell command, show it in a fenced code block and explain what it does. " +
	"Never invent container/file state that isn't in the provided context."

// systemForTask returns the task-specific system prompt.
func systemForTask(task string) string {
	switch task {
	case TaskExplainLog:
		return basePreamble + " The user has selected log lines from a container. Explain what they mean, " +
			"call out errors/warnings, and suggest likely causes."
	case TaskDiagnose:
		return basePreamble + " The user is asking why a container is unhealthy or failing. Use the inspect " +
			"data and recent logs to give a step-by-step diagnosis and the most likely fix."
	case TaskExplainConfig:
		return basePreamble + " The user has selected a config/compose/env file. Explain in plain language what " +
			"it does, section by section, and flag anything risky."
	case TaskSuggestFix:
		return basePreamble + " A permission or access problem has been identified. Propose the minimal, safest " +
			"remediation command(s), and note any side effects."
	default:
		return basePreamble
	}
}

// truncateContext keeps the LAST maxContextLines lines (logs are most relevant
// at the tail) and then enforces a byte cap, prepending an elision marker when
// anything was dropped.
func truncateContext(text string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	dropped := 0
	if len(lines) > maxContextLines {
		dropped = len(lines) - maxContextLines
		lines = lines[len(lines)-maxContextLines:]
	}
	out := strings.Join(lines, "\n")
	if len(out) > maxContextBytes {
		// Slicing at an arbitrary byte offset can split a multi-byte rune; drop
		// the leading partial rune so the context is valid UTF-8.
		out = strings.ToValidUTF8(out[len(out)-maxContextBytes:], "")
		dropped++ // signal a byte-level cut too
	}
	if dropped > 0 {
		return "[earlier context truncated]\n" + out
	}
	return out
}

// buildSystem composes the system prompt: task preamble + the (truncated)
// context block. The user's question stays in AskRequest.Prompt.
func buildSystem(task, context string) string {
	sys := systemForTask(task)
	if c := truncateContext(context); c != "" {
		sys += "\n\n--- Context ---\n" + c
	}
	return sys
}

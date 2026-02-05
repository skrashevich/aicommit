package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	ProviderOpenAI     = "openai"
	ProviderOpenRouter = "openrouter"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_completion_tokens,omitempty"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
	Text    string      `json:"text"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

func generateWithLLM(opts Options, mode Mode, changes []Change, diff string, commitType, scope string, breaking bool, breakingNote, heuristic string, reasons []string) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(opts.LLMProvider))
	if provider == "" {
		provider = ProviderOpenAI
	}
	switch provider {
	case ProviderOpenAI, ProviderOpenRouter:
	default:
		return "", fmt.Errorf("unsupported llm provider: %s", provider)
	}

	model := strings.TrimSpace(opts.LLMModel)
	if model == "" {
		return "", errors.New("llm model is required (use -model or COMMITGEN_LLM_MODEL)")
	}

	endpoint := resolveEndpoint(provider, opts.LLMEndpoint)
	apiKey := resolveAPIKey(provider, opts.LLMKey)
	if apiKey == "" {
		return "", errors.New("llm api key is required (use env or -llm-key)")
	}

	system := strings.TrimSpace(opts.LLMSystem)
	if system == "" {
		system = defaultLLMSystemPrompt()
	}

	user := buildLLMUserPrompt(opts, mode, changes, diff, commitType, scope, breaking, breakingNote, heuristic, reasons)
	if extra := strings.TrimSpace(opts.LLMUser); extra != "" {
		user = user + "\n\nExtra instructions:\n" + extra
	}

	var temp *float64
	if opts.LLMTemperature >= 0 {
		value := opts.LLMTemperature
		temp = &value
	}
	var maxTokens *int
	if opts.LLMMaxTokens > 0 {
		value := opts.LLMMaxTokens
		maxTokens = &value
	}

	payload := chatRequest{
		Model:       model,
		Messages:    []chatMessage{{Role: "system", Content: system}, {Role: "user", Content: user}},
		Temperature: temp,
		MaxTokens:   maxTokens,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	if provider == ProviderOpenRouter {
		if opts.LLMReferer != "" {
			req.Header.Set("HTTP-Referer", opts.LLMReferer)
		}
		if opts.LLMTitle != "" {
			req.Header.Set("X-Title", opts.LLMTitle)
		}
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("llm http %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var response chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", errors.New("llm response has no choices")
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if content == "" {
		content = strings.TrimSpace(response.Choices[0].Text)
	}
	content = cleanLLMMessage(content)
	if content == "" {
		return "", errors.New("llm response content is empty")
	}

	return content, nil
}

func resolveEndpoint(provider string, override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	switch provider {
	case ProviderOpenRouter:
		return "https://openrouter.ai/api/v1/chat/completions"
	default:
		return "https://api.openai.com/v1/chat/completions"
	}
}

func resolveAPIKey(provider string, override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	if env := strings.TrimSpace(os.Getenv("COMMITGEN_LLM_KEY")); env != "" {
		return env
	}
	switch provider {
	case ProviderOpenRouter:
		return strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	default:
		return strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
}

func defaultLLMSystemPrompt() string {
	return strings.Join([]string{
		"You are a commit message generator for git.",
		"Return ONLY the commit message text.",
		"No preface, no analysis, no markdown, no code fences, no surrounding quotes.",
		"Follow the user's formatting and language requirements exactly.",
		"Use only the provided context; do not invent changes.",
	}, " ")
}

func buildLLMUserPrompt(opts Options, mode Mode, changes []Change, diff string, commitType, scope string, breaking bool, breakingNote, heuristic string, reasons []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Requirements:\n")
	fmt.Fprintf(&b, "- Language: %s\n", opts.Lang)
	fmt.Fprintf(&b, "- Format: %s\n", opts.Format)
	if opts.Format == FormatConventional || opts.Format == FormatGitmoji {
		fmt.Fprintf(&b, "- Use format: type(scope)!: subject (scope optional).\n")
	}
	if opts.Format == FormatPlain {
		fmt.Fprintf(&b, "- Use a single-line subject without type prefix.\n")
	}
	fmt.Fprintf(&b, "- Subject max length: %d characters.\n", opts.MaxSubject)
	fmt.Fprintf(&b, "- Body mode: %s.\n", opts.Body)
	fmt.Fprintf(&b, "- For body lists, use '- ' bullet per line.\n")
	if opts.Body == BodyAuto {
		fmt.Fprintf(&b, "- Auto body: if files <= %d, list files; otherwise provide a one-line summary.\n", opts.MaxItems)
	}
	if opts.Emoji || opts.Format == FormatGitmoji {
		fmt.Fprintf(&b, "- Prepend gitmoji code that matches the type (e.g., :sparkles:, :bug:).\n")
	}
	if len(opts.Refs) > 0 {
		fmt.Fprintf(&b, "- Include footer: Refs: %s\n", strings.Join(opts.Refs, ", "))
	}
	if len(opts.Closes) > 0 {
		fmt.Fprintf(&b, "- Include footer: Closes: %s\n", strings.Join(opts.Closes, ", "))
	}
	if breaking {
		if breakingNote == "" {
			fmt.Fprintf(&b, "- Breaking change detected. Add 'BREAKING CHANGE: ...' footer.\n")
		} else {
			fmt.Fprintf(&b, "- Breaking change detected (%s). Add 'BREAKING CHANGE: %s' footer.\n", breakingNote, breakingNote)
		}
	} else {
		fmt.Fprintf(&b, "- No breaking change detected; avoid BREAKING CHANGE unless diff clearly requires it.\n")
	}

	fmt.Fprintf(&b, "\nContext:\n")
	fmt.Fprintf(&b, "- Mode: %s\n", mode)
	fmt.Fprintf(&b, "- Heuristic suggestion: %s\n", oneLine(heuristic))
	if commitType != "" {
		fmt.Fprintf(&b, "- Heuristic type: %s\n", commitType)
	}
	if scope != "" {
		fmt.Fprintf(&b, "- Heuristic scope: %s\n", scope)
	}
	if len(reasons) > 0 {
		fmt.Fprintf(&b, "- Heuristic reasons: %s\n", strings.Join(reasons, "; "))
	}

	fmt.Fprintf(&b, "\nChanges:\n")
	fileLines := buildFileLines(changes, minInt(opts.MaxItems, 20), opts.Lang)
	if len(fileLines) == 0 {
		fmt.Fprintf(&b, "- (no files)\n")
	} else {
		for _, line := range fileLines {
			fmt.Fprintf(&b, "%s\n", line)
		}
	}

	stats, _ := collectNumstat(mode)
	if len(stats) > 0 {
		fmt.Fprintf(&b, "\nStats:\n")
		for _, line := range buildStatLines(stats, minInt(opts.MaxItems, 20), opts.Lang) {
			fmt.Fprintf(&b, "%s\n", line)
		}
	}

	trimmedDiff, truncated := truncateDiff(diff, opts.LLMMaxDiff)
	if strings.TrimSpace(trimmedDiff) != "" {
		if truncated {
			fmt.Fprintf(&b, "\nDiff (truncated to %d bytes):\n", opts.LLMMaxDiff)
		} else {
			fmt.Fprintf(&b, "\nDiff:\n")
		}
		fmt.Fprintln(&b, trimmedDiff)
	}

	return strings.TrimSpace(b.String())
}

func truncateDiff(diff string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(diff) <= maxBytes {
		return diff, false
	}
	return diff[:maxBytes], true
}

func cleanLLMMessage(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSpace(s)
		if idx := strings.Index(s, "\n"); idx != -1 {
			first := strings.TrimSpace(s[:idx])
			if len(first) > 0 && len(first) <= 12 && !strings.Contains(first, " ") {
				s = strings.TrimSpace(s[idx+1:])
			}
		}
		if end := strings.LastIndex(s, "```"); end != -1 {
			s = strings.TrimSpace(s[:end])
		}
	}

	lower := strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(lower, "commit message:") {
		s = strings.TrimSpace(s[len("commit message:"):])
	}
	if strings.HasPrefix(lower, "message:") {
		s = strings.TrimSpace(s[len("message:"):])
	}

	s = strings.Trim(s, "\"`")
	return strings.TrimSpace(s)
}

func oneLine(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}

func minInt(a, b int) int {
	if a <= 0 {
		return b
	}
	if a < b {
		return a
	}
	return b
}

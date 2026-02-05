package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	opts := parseFlags()
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func parseFlags() Options {
	var opts Options

	formatDefault := envOrDefault("COMMITGEN_FORMAT", string(FormatConventional))
	langDefault := envOrDefault("COMMITGEN_LANG", "auto")
	bodyDefault := envOrDefault("COMMITGEN_BODY", string(BodyAuto))
	maxItemsDefault := envOrInt("COMMITGEN_MAX_ITEMS", 8)
	maxSubjectDefault := envOrInt("COMMITGEN_MAX_SUBJECT", 72)
	typeDefault := envOrDefault("COMMITGEN_TYPE", "")
	scopeDefault := envOrDefault("COMMITGEN_SCOPE", "")
	refsDefault := envOrDefault("COMMITGEN_REFS", "")
	closesDefault := envOrDefault("COMMITGEN_CLOSES", "")
	llmDefault := envOrBool("COMMITGEN_LLM", false)
	llmProviderDefault := envOrDefault("COMMITGEN_LLM_PROVIDER", "")
	llmModelDefault := envOrDefault("COMMITGEN_LLM_MODEL", "gpt-5-nano")
	llmEndpointDefault := envOrDefault("COMMITGEN_LLM_ENDPOINT", "")
	llmKeyDefault := envOrDefault("COMMITGEN_LLM_KEY", "")
	llmTemperatureDefault := envOrFloat("COMMITGEN_LLM_TEMPERATURE", 1)
	llmMaxTokensDefault := envOrInt("COMMITGEN_LLM_MAX_TOKENS", 300)
	llmMaxDiffDefault := envOrInt("COMMITGEN_LLM_MAX_DIFF", 20000)
	llmStrictDefault := envOrBool("COMMITGEN_LLM_STRICT", false)
	llmSystemDefault := envOrDefault("COMMITGEN_LLM_SYSTEM", "")
	llmUserDefault := envOrDefault("COMMITGEN_LLM_USER", "")
	llmRefererDefault := envOrDefault("COMMITGEN_OPENROUTER_REFERER", "")
	llmTitleDefault := envOrDefault("COMMITGEN_OPENROUTER_TITLE", "aicommit")

	var modeFlag string
	var formatFlag string
	var langFlag string
	var typeFlag string
	var scopeFlag string
	var bodyFlag string
	var refsFlag string
	var closesFlag string
	var stagedFlag bool
	var unstagedFlag bool
	var allFlag bool
	var breakingFlag bool
	var emojiFlag bool
	var explainFlag bool
	var copyFlag bool
	var maxItemsFlag int
	var maxSubjectFlag int
	var llmFlag bool
	var llmProviderFlag string
	var llmModelFlag string
	var llmEndpointFlag string
	var llmKeyFlag string
	var llmTemperatureFlag float64
	var llmMaxTokensFlag int
	var llmMaxDiffFlag int
	var llmStrictFlag bool
	var llmSystemFlag string
	var llmUserFlag string
	var llmRefererFlag string
	var llmTitleFlag string

	flag.StringVar(&modeFlag, "mode", "", "auto|staged|unstaged|all")
	flag.BoolVar(&stagedFlag, "staged", false, "use staged changes")
	flag.BoolVar(&unstagedFlag, "unstaged", false, "use unstaged changes")
	flag.BoolVar(&allFlag, "all", false, "use staged and unstaged changes")
	flag.StringVar(&formatFlag, "format", formatDefault, "plain|conventional|gitmoji")
	flag.StringVar(&langFlag, "lang", langDefault, "auto|en|ru")
	flag.StringVar(&typeFlag, "type", typeDefault, "force commit type")
	flag.StringVar(&scopeFlag, "scope", scopeDefault, "force scope")
	flag.BoolVar(&breakingFlag, "breaking", false, "mark as breaking change")
	flag.StringVar(&bodyFlag, "body", bodyDefault, "auto|none|files|stats|summary")
	flag.IntVar(&maxItemsFlag, "max-items", maxItemsDefault, "max items in body list")
	flag.IntVar(&maxSubjectFlag, "max-subject", maxSubjectDefault, "max subject length")
	flag.StringVar(&refsFlag, "refs", refsDefault, "comma-separated issue references")
	flag.StringVar(&closesFlag, "closes", closesDefault, "comma-separated issue numbers to close")
	flag.BoolVar(&emojiFlag, "emoji", false, "prepend gitmoji code to subject")
	flag.BoolVar(&explainFlag, "explain", false, "print reasoning to stderr")
	flag.BoolVar(&copyFlag, "copy", false, "copy result to clipboard if possible")
	flag.BoolVar(&llmFlag, "llm", llmDefault, "use LLM to generate message")
	flag.StringVar(&llmProviderFlag, "provider", llmProviderDefault, "openai|openrouter")
	flag.StringVar(&llmModelFlag, "model", llmModelDefault, "LLM model name")
	flag.StringVar(&llmEndpointFlag, "endpoint", llmEndpointDefault, "override LLM endpoint URL")
	flag.StringVar(&llmKeyFlag, "llm-key", llmKeyDefault, "LLM API key (prefer env)")
	flag.Float64Var(&llmTemperatureFlag, "temperature", llmTemperatureDefault, "LLM sampling temperature")
	flag.IntVar(&llmMaxTokensFlag, "max-tokens", llmMaxTokensDefault, "LLM max tokens")
	flag.IntVar(&llmMaxDiffFlag, "llm-max-diff", llmMaxDiffDefault, "max diff bytes to send to LLM")
	flag.BoolVar(&llmStrictFlag, "llm-strict", llmStrictDefault, "fail if LLM request fails")
	flag.StringVar(&llmSystemFlag, "llm-system", llmSystemDefault, "override LLM system prompt")
	flag.StringVar(&llmUserFlag, "llm-user", llmUserDefault, "extra LLM user instructions")
	flag.StringVar(&llmRefererFlag, "llm-referer", llmRefererDefault, "openrouter HTTP-Referer")
	flag.StringVar(&llmTitleFlag, "llm-title", llmTitleDefault, "openrouter X-Title")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Generate a commit message from current git changes.")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		flag.PrintDefaults()
	}

	flag.Parse()

	opts.Mode = ModeAuto
	if allFlag {
		opts.Mode = ModeAll
	} else if stagedFlag {
		opts.Mode = ModeStaged
	} else if unstagedFlag {
		opts.Mode = ModeUnstaged
	}
	if modeFlag != "" {
		opts.Mode = Mode(modeFlag)
	}

	opts.Format = Format(formatFlag)
	opts.Lang = langFlag
	opts.Type = strings.TrimSpace(typeFlag)
	opts.Scope = strings.TrimSpace(scopeFlag)
	opts.Breaking = breakingFlag
	opts.Body = BodyMode(bodyFlag)
	opts.MaxItems = maxItemsFlag
	opts.MaxSubject = maxSubjectFlag
	opts.Refs = splitList(refsFlag)
	opts.Closes = splitList(closesFlag)
	opts.Emoji = emojiFlag
	opts.Explain = explainFlag
	opts.Copy = copyFlag
	opts.LLMEnabled = llmFlag
	opts.LLMProvider = strings.TrimSpace(llmProviderFlag)
	opts.LLMModel = strings.TrimSpace(llmModelFlag)
	opts.LLMEndpoint = strings.TrimSpace(llmEndpointFlag)
	opts.LLMKey = strings.TrimSpace(llmKeyFlag)
	opts.LLMTemperature = llmTemperatureFlag
	opts.LLMMaxTokens = llmMaxTokensFlag
	opts.LLMMaxDiff = llmMaxDiffFlag
	opts.LLMStrict = llmStrictFlag
	opts.LLMSystem = strings.TrimSpace(llmSystemFlag)
	opts.LLMUser = strings.TrimSpace(llmUserFlag)
	opts.LLMReferer = strings.TrimSpace(llmRefererFlag)
	opts.LLMTitle = strings.TrimSpace(llmTitleFlag)

	return opts
}

func run(opts Options) error {
	if err := ensureGit(); err != nil {
		return err
	}
	if opts.MaxItems <= 0 {
		opts.MaxItems = 8
	}
	if opts.MaxSubject <= 0 {
		opts.MaxSubject = 72
	}
	if opts.Mode == "" {
		opts.Mode = ModeAuto
	}
	if opts.LLMEnabled && opts.LLMMaxDiff <= 0 {
		opts.LLMMaxDiff = 20000
	}
	if opts.Lang == "auto" || opts.Lang == "" {
		opts.Lang = detectLang()
	}
	if opts.Lang != "en" && opts.Lang != "ru" {
		return fmt.Errorf("unsupported lang: %s", opts.Lang)
	}
	if !validFormat(opts.Format) {
		return fmt.Errorf("unsupported format: %s", opts.Format)
	}
	if !validBody(opts.Body) {
		return fmt.Errorf("unsupported body mode: %s", opts.Body)
	}
	if !validMode(opts.Mode) {
		return fmt.Errorf("unsupported mode: %s", opts.Mode)
	}

	if _, err := gitOutput("rev-parse", "--show-toplevel"); err != nil {
		return errors.New("not a git repository")
	}

	staged, unstaged, err := collectChanges()
	if err != nil {
		return err
	}
	modeUsed, changes := selectChanges(opts.Mode, staged, unstaged)
	if len(changes) == 0 {
		return fmt.Errorf("no changes found for mode %s", modeUsed)
	}

	diff, _ := collectDiff(modeUsed)

	commitType, reasons := detectType(changes, diff, opts)
	scope := detectScope(changes, opts.Scope)
	breaking, breakingNote := detectBreaking(changes, diff, opts)
	subject := buildSubject(commitType, scope, changes, opts)
	body := buildBody(changes, modeUsed, opts, breaking, breakingNote)
	message := formatMessage(commitType, scope, subject, body, opts, breaking)

	llmUsed := false
	if opts.LLMEnabled {
		llmMessage, err := generateWithLLM(opts, modeUsed, changes, diff, commitType, scope, breaking, breakingNote, message, reasons)
		if err != nil {
			if opts.LLMStrict {
				return err
			}
			fmt.Fprintln(os.Stderr, "llm failed, using heuristic:", err)
		} else if llmMessage != "" {
			message = llmMessage
			llmUsed = true
		}
	}

	fmt.Println(message)

	if opts.Copy {
		if err := copyToClipboard(message); err != nil {
			fmt.Fprintln(os.Stderr, "copy failed:", err)
		}
	}
	if opts.Explain {
		printExplain(os.Stderr, opts, modeUsed, commitType, scope, breaking, llmUsed, reasons, changes)
	}

	return nil
}

func envOrDefault(key, def string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	return val
}

func envOrInt(key string, def int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return parsed
}

func envOrBool(key string, def bool) bool {
	val := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if val == "" {
		return def
	}
	switch val {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

func envOrFloat(key string, def float64) float64 {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return def
	}
	return parsed
}

func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

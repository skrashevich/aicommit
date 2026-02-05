package main

type Mode string

type Format string

type BodyMode string

const (
	ModeAuto     Mode = "auto"
	ModeStaged   Mode = "staged"
	ModeUnstaged Mode = "unstaged"
	ModeAll      Mode = "all"
)

const (
	FormatConventional Format = "conventional"
	FormatPlain        Format = "plain"
	FormatGitmoji      Format = "gitmoji"
)

const (
	BodyAuto    BodyMode = "auto"
	BodyNone    BodyMode = "none"
	BodyFiles   BodyMode = "files"
	BodyStats   BodyMode = "stats"
	BodySummary BodyMode = "summary"
)

type Options struct {
	Mode       Mode
	Format     Format
	Lang       string
	Type       string
	Scope      string
	Breaking   bool
	Body       BodyMode
	MaxItems   int
	MaxSubject int
	Emoji      bool
	Explain    bool
	Copy       bool
	Refs       []string
	Closes     []string
	LLMEnabled     bool
	LLMProvider    string
	LLMModel       string
	LLMEndpoint    string
	LLMKey         string
	LLMTemperature float64
	LLMMaxTokens   int
	LLMMaxDiff     int
	LLMStrict      bool
	LLMSystem      string
	LLMUser        string
	LLMReferer     string
	LLMTitle       string
}

type Change struct {
	Path    string
	OldPath string
	Status  string
	Source  Mode
}

type FileStat struct {
	Path    string
	Added   int
	Deleted int
	Binary  bool
}

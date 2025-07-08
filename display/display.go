package display

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type Color string

const (
	Reset Color = "\033[0m"
	Bold  Color = "\033[1m"
	Dim   Color = "\033[2m"

	Black   Color = "\033[30m"
	Red     Color = "\033[31m"
	Green   Color = "\033[32m"
	Yellow  Color = "\033[33m"
	Blue    Color = "\033[34m"
	Magenta Color = "\033[35m"
	Cyan    Color = "\033[36m"
	White   Color = "\033[37m"

	BrightBlack   Color = "\033[90m"
	BrightRed     Color = "\033[91m"
	BrightGreen   Color = "\033[92m"
	BrightYellow  Color = "\033[93m"
	BrightBlue    Color = "\033[94m"
	BrightMagenta Color = "\033[95m"
	BrightCyan    Color = "\033[96m"
	BrightWhite   Color = "\033[97m"

	BgRed   Color = "\033[41m"
	BgGreen Color = "\033[42m"
	BgBlue  Color = "\033[44m"
)

type Style struct {
	color      Color
	background Color
	bold       bool
	dim        bool
}

var (
	BranchStyle     = Style{color: BrightCyan, bold: true}
	RepoStyle       = Style{color: BrightBlue, bold: true}

	StagedStyle     = Style{color: Green, bold: true}
	UnstagedStyle   = Style{color: Red, bold: true}
	UntrackedStyle  = Style{color: BrightRed, bold: true}
	ModifiedStyle   = Style{color: Yellow, bold: true}
	DeletedStyle    = Style{color: Red, bold: true}
	AddedStyle      = Style{color: Green, bold: true}
	RenamedStyle    = Style{color: Magenta, bold: true}

	DiffHeaderStyle = Style{color: BrightWhite, bold: true}
	DiffAddedStyle  = Style{color: Green}
	DiffRemovedStyle = Style{color: Red}
	DiffContextStyle = Style{color: White}
	DiffPathStyle   = Style{color: BrightYellow, bold: true}

	SuccessStyle    = Style{color: Green, bold: true}
	WarningStyle    = Style{color: Yellow, bold: true}
	ErrorStyle      = Style{color: Red, bold: true}
	InfoStyle       = Style{color: Cyan}
	HintStyle       = Style{color: BrightBlack, dim: true}

	ProgressStyle   = Style{color: BrightCyan}
	HashStyle       = Style{color: Yellow}

	EmphasisStyle   = Style{color: BrightWhite, bold: true}
	SecondaryStyle  = Style{color: BrightBlack}
)

type Spinner struct {
	formatter *Formatter
	chars     []string
	delay     time.Duration
	message   string
	active    bool
}

type Formatter struct {
	writer       io.Writer
	colorEnabled bool
}

func NewFormatter(w io.Writer) *Formatter {
	if w == nil {
		w = os.Stdout
	}
	return &Formatter{
		writer:       w,
		colorEnabled: isTerminalColorSupported(w),
	}
}

func NewFormatterWithColor(w io.Writer, colorEnabled bool) *Formatter {
	if w == nil {
		w = os.Stdout
	}
	return &Formatter{
		writer:       w,
		colorEnabled: colorEnabled,
	}
}

func (f *Formatter) SetColorEnabled(enabled bool) { f.colorEnabled = enabled }
func (f *Formatter) IsColorEnabled() bool         { return f.colorEnabled }
func (f *Formatter) Apply(style Style, text string) string {
	if !f.colorEnabled {
		return text
	}

	var codes []string

	if style.bold {
		codes = append(codes, string(Bold))
	}
	if style.dim {
		codes = append(codes, string(Dim))
	}
	if style.color != "" {
		codes = append(codes, string(style.color))
	}
	if style.background != "" {
		codes = append(codes, string(style.background))
	}

	if len(codes) == 0 {
		return text
	}

	return strings.Join(codes, "") + text + string(Reset)
}

func (f *Formatter) Print(text string, style ...Style) {
	if len(style) > 0 {
		text = f.Apply(style[0], text)
	}
	fmt.Fprint(f.writer, text)
}

func (f *Formatter) Println(text string, style ...Style) {
	if len(style) > 0 {
		text = f.Apply(style[0], text)
	}
	fmt.Fprintln(f.writer, text)
}

func (f *Formatter) Printf(format string, args ...interface{}) {
	fmt.Fprintf(f.writer, format, args...)
}

func (f *Formatter) Printlnf(style Style, format string, args ...interface{}) {
	text := fmt.Sprintf(format, args...)
	f.Println(text, style)
}

func (f *Formatter) Branch(branch string) string { return f.Apply(BranchStyle, branch) }
func (f *Formatter) Hash(hash string) string {
	if len(hash) > 7 {
		hash = hash[:7]
	}
	return f.Apply(HashStyle, hash)
}
func (f *Formatter) Path(path string) string       { return f.Apply(DiffPathStyle, path) }
func (f *Formatter) Success(message string) string { return f.Apply(SuccessStyle, message) }
func (f *Formatter) Warning(message string) string { return f.Apply(WarningStyle, message) }
func (f *Formatter) Error(message string) string   { return f.Apply(ErrorStyle, message) }
func (f *Formatter) Info(message string) string    { return f.Apply(InfoStyle, message) }
func (f *Formatter) Hint(message string) string    { return f.Apply(HintStyle, message) }
func (f *Formatter) Emphasis(text string) string   { return f.Apply(EmphasisStyle, text) }
func (f *Formatter) Secondary(text string) string  { return f.Apply(SecondaryStyle, text) }

func (f *Formatter) Progress(message string) { f.Print(f.Apply(ProgressStyle, "● ") + message) }
func (f *Formatter) ProgressDone(message string) { f.Println(f.Apply(SuccessStyle, "✓ ") + message) }
func (f *Formatter) ProgressFail(message string) { f.Println(f.Apply(ErrorStyle, "✗ ") + message) }

func (f *Formatter) NewSpinner(message string) *Spinner {
	return &Spinner{
		formatter: f,
		chars:     []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		delay:     100 * time.Millisecond,
		message:   message,
	}
}

func (s *Spinner) Start() {
	s.active = true
	go func() {
		i := 0
		for s.active {
			char := s.chars[i%len(s.chars)]
			s.formatter.Printf("\r%s %s",
				s.formatter.Apply(ProgressStyle, char),
				s.message)
			time.Sleep(s.delay)
			i++
		}
	}()
}

func (s *Spinner) Stop() {
	s.active = false
	s.formatter.Printf("\r%s\r", strings.Repeat(" ", len(s.message)+2))
}

func (s *Spinner) Success(message string) { s.Stop(); s.formatter.ProgressDone(message) }
func (s *Spinner) Fail(message string)    { s.Stop(); s.formatter.ProgressFail(message) }

var defaultFormatter = NewFormatter(os.Stdout)

func Print(text string, style ...Style)         { defaultFormatter.Print(text, style...) }
func Println(text string, style ...Style)       { defaultFormatter.Println(text, style...) }
func Printf(format string, args ...interface{}) { defaultFormatter.Printf(format, args...) }
func Printlnf(style Style, format string, args ...interface{}) { defaultFormatter.Printlnf(style, format, args...) }

func Branch(branch string) string    { return defaultFormatter.Branch(branch) }
func Hash(hash string) string        { return defaultFormatter.Hash(hash) }
func Path(path string) string        { return defaultFormatter.Path(path) }
func Success(message string) string  { return defaultFormatter.Success(message) }
func Warning(message string) string  { return defaultFormatter.Warning(message) }
func Error(message string) string    { return defaultFormatter.Error(message) }
func Info(message string) string     { return defaultFormatter.Info(message) }
func Hint(message string) string     { return defaultFormatter.Hint(message) }
func Emphasis(text string) string    { return defaultFormatter.Emphasis(text) }
func Secondary(text string) string   { return defaultFormatter.Secondary(text) }

func Progress(message string)        { defaultFormatter.Progress(message) }
func ProgressDone(message string)    { defaultFormatter.ProgressDone(message) }
func ProgressFail(message string)    { defaultFormatter.ProgressFail(message) }
func NewSpinner(message string) *Spinner { return defaultFormatter.NewSpinner(message) }

func SetColorEnabled(enabled bool) { defaultFormatter.SetColorEnabled(enabled) }
func IsColorEnabled() bool         { return defaultFormatter.IsColorEnabled() }

func isTerminalColorSupported(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		if f == os.Stdout || f == os.Stderr {
			term := os.Getenv("TERM")
			if term == "" || term == "dumb" {
				return false
			}

			if os.Getenv("NO_COLOR") != "" {
				return false
			}

			if os.Getenv("FORCE_COLOR") != "" {
				return true
			}

			return strings.Contains(term, "color") ||
				   strings.Contains(term, "xterm") ||
				   strings.Contains(term, "screen") ||
				   strings.Contains(term, "tmux") ||
				   term == "ansi"
		}
	}

	return false
}

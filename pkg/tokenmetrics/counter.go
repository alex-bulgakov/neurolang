package tokenmetrics

import (
	"regexp"
	"strings"
	"unicode"
)

// MetricResult stores comparison metrics between code formats
type MetricResult struct {
	Language   string
	CharCount  int
	LineCount  int
	TokenCount int
}

// EstimateTokens calculates approximate BPE tokens (similar to cl100k_base / o200k_base)
// used by modern LLMs (OpenAI GPT-4o, Claude, Gemini).
func EstimateTokens(code string) int {
	if len(strings.TrimSpace(code)) == 0 {
		return 0
	}

	// Regex pattern mimicking GPT-4 / Claude code tokenizer:
	// 1. Identifiers / Words: [a-zA-Z_]+
	// 2. Numbers: [0-9]+
	// 3. Operators and Punctuation (each is typically 1 token): [|?@!&+\-*/=<>:;,.\(\)\[\]\{\}\^~%]
	// 4. Strings: quoted sequences
	// 5. Indents / Newlines: \n, \t, spaces
	re := regexp.MustCompile(`(?i)'s|'t|'re|'ve|'m|'ll|'d|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}{1,3}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+([^\s]+)?| +`)

	matches := re.FindAllString(code, -1)
	count := 0

	for _, match := range matches {
		trimmed := strings.TrimSpace(match)
		if len(trimmed) == 0 {
			// Whitespace sequence (indentation or newlines)
			if strings.Contains(match, "\n") {
				count += strings.Count(match, "\n")
			} else if len(match) >= 4 {
				count += len(match) / 4
			}
			continue
		}

		// Split compound camelCase or snake_case
		if isCompoundWord(trimmed) {
			parts := splitSubwords(trimmed)
			count += len(parts)
		} else {
			count++
		}
	}

	if count == 0 {
		return 1
	}
	return count
}

func isCompoundWord(s string) bool {
	if strings.Contains(s, "_") {
		return true
	}
	hasLower := false
	for _, r := range s {
		if unicode.IsLower(r) {
			hasLower = true
		} else if hasLower && unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func splitSubwords(s string) []string {
	var parts []string
	var current strings.Builder

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '_' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}
		if unicode.IsUpper(r) && current.Len() > 0 && (i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
			parts = append(parts, current.String())
			current.Reset()
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func Analyze(code, langName string) MetricResult {
	lines := strings.Split(code, "\n")
	return MetricResult{
		Language:   langName,
		CharCount:  len(code),
		LineCount:  len(lines),
		TokenCount: EstimateTokens(code),
	}
}

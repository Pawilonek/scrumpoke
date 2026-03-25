package game

import (
	_ "embed"
	"encoding/json"
	"strings"
	"unicode"
)

//go:embed adjectives.json
var adjectivesJSON []byte

//go:embed nouns.json
var nounsJSON []byte

var embeddedAdjectives, embeddedNouns []string

func init() {
	if err := json.Unmarshal(adjectivesJSON, &embeddedAdjectives); err != nil {
		panic("game: adjectives.json: " + err.Error())
	}
	if err := json.Unmarshal(nounsJSON, &embeddedNouns); err != nil {
		panic("game: nouns.json: " + err.Error())
	}
}

func slugifyWord(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-':
			b.WriteRune(r)
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return strings.Trim(out, "-")
}

func normalizeNounForPlayerName(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "'", "")
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune(' ')
		}
	}
	return strings.TrimSpace(doubleSpaceRe.ReplaceAllString(b.String(), " "))
}

func titlePlayerName(s string) string {
	s = strings.ToLower(s)
	parts := strings.Fields(s)
	for i, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		r[0] = unicode.ToUpper(r[0])
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}

package game

import (
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	roomSlugRe    = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)
	playerNameRe  = regexp.MustCompile(`^[A-Za-z0-9 ]+$`)
	cardsTokenRe  = regexp.MustCompile(`^[0-9]+(/[0-9]+)?$`)
	doubleSpaceRe = regexp.MustCompile(`\s+`)
)

func ValidateRoomSlug(slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return errors.New("empty room slug")
	}
	if !roomSlugRe.MatchString(slug) {
		return errors.New("invalid room slug")
	}
	return nil
}

// ValidatePlayerName allows only letters, digits and spaces.
// It also normalizes consecutive whitespace into a single space and trims ends.
func ValidatePlayerName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("empty player name")
	}
	name = doubleSpaceRe.ReplaceAllString(name, " ")
	if len(name) > 32 {
		return "", errors.New("player name too long")
	}
	if !playerNameRe.MatchString(name) {
		return "", errors.New("invalid player name")
	}
	return name, nil
}

// ParseCardsTokens parses comma-separated card tokens (e.g. "0, 1/2, 1").
// Duplicates are removed (stable order).
func ParseCardsTokens(cardsText string) ([]string, error) {
	parts := strings.Split(cardsText, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, p := range parts {
		tok := strings.TrimSpace(p)
		if tok == "" {
			continue
		}
		if !cardsTokenRe.MatchString(tok) {
			return nil, errors.New("invalid card token: " + tok)
		}
		// Reject "a/b" where denominator is 0? Not specified; accept as-is.
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}

	if len(out) == 0 {
		return nil, errors.New("no cards")
	}
	return out, nil
}

// CardsTokensDefault returns the default planning poker sequence.
func CardsTokensDefault() []string {
	return []string{"0", "1/2", "1", "2", "3", "5", "8", "13"}
}

func SortUniqueIntsStable(a []int) []int {
	if len(a) == 0 {
		return a
	}
	m := map[int]struct{}{}
	uniq := make([]int, 0, len(a))
	for _, v := range a {
		if _, ok := m[v]; ok {
			continue
		}
		m[v] = struct{}{}
		uniq = append(uniq, v)
	}
	sort.Ints(uniq)
	return uniq
}

func ParseUUIDFromLoose(s string) string {
	// Left as a placeholder if we ever want to accept non-standard inputs.
	// Currently UUID validation is handled elsewhere.
	_ = strconv.IntSize
	return strings.TrimSpace(s)
}

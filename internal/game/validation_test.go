package game

import "testing"

func TestValidateRoomSlug(t *testing.T) {
	ok := []string{"happy-bunny", "a1", "abc-123", "a-b-c-1"}
	for _, s := range ok {
		if err := ValidateRoomSlug(s); err != nil {
			t.Fatalf("expected ok room slug %q: %v", s, err)
		}
	}

	bad := []string{"", "Happy Bunny", "UPPER", "bad_", "nope!", "toolong_" + string(make([]byte, 200))}
	for _, s := range bad {
		if err := ValidateRoomSlug(s); err == nil {
			t.Fatalf("expected invalid room slug %q", s)
		}
	}
}

func TestValidatePlayerName(t *testing.T) {
	ok := []string{"Alice 1", "Bob2", "A B C 123", "  Alice  " }
	for _, s := range ok {
		if _, err := ValidatePlayerName(s); err != nil {
			t.Fatalf("expected ok player name %q: %v", s, err)
		}
	}

	bad := []string{"", "   ", "Alice-1", "Alice_1", "Alice!", "Alice@1", "###"}
	for _, s := range bad {
		if _, err := ValidatePlayerName(s); err == nil {
			t.Fatalf("expected invalid player name %q", s)
		}
	}
}

func TestParseCardsTokens(t *testing.T) {
	cards, err := ParseCardsTokens("0, 1/2, 1, 2, 3, 5, 8, 13")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cards) != 8 {
		t.Fatalf("expected 8 cards, got %d", len(cards))
	}

	// Duplicates removed (stable order preserved).
	cards, err = ParseCardsTokens("0, 0, 1, 1/2, 1/2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cards) != 3 {
		t.Fatalf("expected 3 unique cards, got %d", len(cards))
	}

	_, err = ParseCardsTokens("a, 1/2")
	if err == nil {
		t.Fatalf("expected error for invalid token")
	}
}


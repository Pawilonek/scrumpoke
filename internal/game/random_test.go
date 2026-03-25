package game

import "testing"

func TestRandomRoomSlugValid(t *testing.T) {
	for range 200 {
		s := RandomRoomSlug()
		if err := ValidateRoomSlug(s); err != nil {
			t.Fatalf("RandomRoomSlug %q: %v", s, err)
		}
	}
}

func TestRandomPlayerNameValid(t *testing.T) {
	for range 200 {
		s := RandomPlayerName()
		if _, err := ValidatePlayerName(s); err != nil {
			t.Fatalf("RandomPlayerName %q: %v", s, err)
		}
	}
}

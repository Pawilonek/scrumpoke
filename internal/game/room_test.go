package game

import (
	"testing"
	"time"
)

func TestRoomAutoRevealWhenAllConnectedVoted(t *testing.T) {
	events := make(chan RoomSnapshot, 10)
	r := NewRoom("happy-bunny", []string{"0", "1/2", "1"}, 30*time.Second, events)

	r.UpsertPlayer("u1", "Alice")
	r.UpsertPlayer("u2", "Bob")
	r.SetConnected("u1", true)
	r.SetConnected("u2", true)

	if r.revealed {
		t.Fatalf("expected revealed=false initially")
	}

	if err := r.CastVote("u1", "1/2"); err != nil {
		t.Fatalf("unexpected vote error: %v", err)
	}
	if r.revealed {
		t.Fatalf("expected not revealed after one vote")
	}

	if err := r.CastVote("u2", "0"); err != nil {
		t.Fatalf("unexpected vote error: %v", err)
	}
	if !r.revealed {
		t.Fatalf("expected revealed=true after all votes")
	}
}

func TestRoomStartNewVotingResetsVotesKeepsCards(t *testing.T) {
	events := make(chan RoomSnapshot, 10)
	r := NewRoom("room", []string{"0", "1"}, 30*time.Second, events)

	r.UpsertPlayer("u1", "Alice")
	r.UpsertPlayer("u2", "Bob")
	r.SetConnected("u1", true)
	r.SetConnected("u2", true)

	_ = r.CastVote("u1", "0")
	_ = r.CastVote("u2", "1")
	if !r.revealed {
		t.Fatalf("expected revealed")
	}

	if err := r.StartNewVoting(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.revealed {
		t.Fatalf("expected revealed=false after start new voting")
	}
	if len(r.cards) != 2 || r.cards[0] != "0" || r.cards[1] != "1" {
		t.Fatalf("expected cards preserved, got: %#v", r.cards)
	}
	for _, p := range r.players {
		if p.Vote != nil {
			t.Fatalf("expected vote cleared for %s", p.UUID)
		}
	}
}

func TestRoomUpdateCardsResetsVotesAndReveal(t *testing.T) {
	events := make(chan RoomSnapshot, 10)
	r := NewRoom("room", []string{"0", "1"}, 30*time.Second, events)

	r.UpsertPlayer("u1", "Alice")
	r.UpsertPlayer("u2", "Bob")
	r.SetConnected("u1", true)
	r.SetConnected("u2", true)

	_ = r.CastVote("u1", "0")
	_ = r.CastVote("u2", "1")
	if !r.revealed {
		t.Fatalf("expected revealed")
	}

	r.UpdateCards([]string{"0", "1/2"})
	if r.revealed {
		t.Fatalf("expected revealed=false after cards update")
	}
	if len(r.cards) != 2 || r.cards[1] != "1/2" {
		t.Fatalf("expected cards updated, got: %#v", r.cards)
	}
	for _, p := range r.players {
		if p.Vote != nil {
			t.Fatalf("expected votes cleared for %s", p.UUID)
		}
	}
}

func TestRoomRevealNowAllowsBeforeEveryoneVoted(t *testing.T) {
	events := make(chan RoomSnapshot, 10)
	r := NewRoom("room", []string{"0", "1"}, 30*time.Second, events)

	r.UpsertPlayer("u1", "Alice")
	r.UpsertPlayer("u2", "Bob")
	r.SetConnected("u1", true)
	r.SetConnected("u2", true)

	_ = r.CastVote("u1", "0")
	if err := r.RevealNow(); err != nil {
		t.Fatalf("expected reveal to succeed before all voted: %v", err)
	}
	if !r.revealed {
		t.Fatalf("expected revealed=true")
	}
}


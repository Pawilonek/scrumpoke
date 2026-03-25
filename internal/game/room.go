package game

import (
	"errors"
	"sort"
	"sync"
	"time"
)

type Player struct {
	UUID string
	Name string

	Vote       *string
	Connected  bool
	disconnAt  time.Time
	seqCreated int
}

// PlayerSnapshot includes enough info for the transport to tailor per-client messages.
type PlayerSnapshot struct {
	UUID      string  `json:"uuid"`
	Name      string  `json:"name"`
	Voted     bool    `json:"voted"`
	VoteCard  *string `json:"card,omitempty"`
	Connected bool    `json:"connected"`
}

type RoomSnapshot struct {
	Room     string            `json:"room"`
	Cards    []string          `json:"cards"`
	Revealed bool              `json:"revealed"`
	CanReveal bool             `json:"canReveal"`
	Players  []PlayerSnapshot  `json:"players"`
}

// Room is the in-memory game state machine.
// It publishes snapshots to events sink on every state change.
type Room struct {
	slug string

	mu       sync.Mutex
	cards    []string
	revealed bool

	players   map[string]*Player
	nextSeq    int

	gracePeriod time.Duration
	events       chan<- RoomSnapshot
}

func NewRoom(slug string, initialCards []string, gracePeriod time.Duration, events chan<- RoomSnapshot) *Room {
	return &Room{
		slug:          slug,
		cards:         append([]string(nil), initialCards...),
		revealed:     false,
		players:      make(map[string]*Player),
		nextSeq:      1,
		gracePeriod:  gracePeriod,
		events:       events,
	}
}

func (r *Room) Snapshot() RoomSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

func (r *Room) snapshotLocked() RoomSnapshot {
	players := make([]PlayerSnapshot, 0, len(r.players))

	for _, p := range r.players {
		var voteCard *string
		if r.revealed || p.Vote != nil {
			// Transport will decide whether to expose card pre-reveal for others.
			// We include it here; per-client filtering happens later.
			if p.Vote != nil {
				v := *p.Vote
				voteCard = &v
			}
		}
		players = append(players, PlayerSnapshot{
			UUID:      p.UUID,
			Name:      p.Name,
			Voted:     p.Vote != nil,
			VoteCard:  voteCard,
			Connected: p.Connected,
		})
	}

	sort.Slice(players, func(i, j int) bool {
		// Stable ordering by created sequence to make UI deterministic.
		pi := r.players[players[i].UUID].seqCreated
		pj := r.players[players[j].UUID].seqCreated
		return pi < pj
	})

	// Manual "Reveal cards" is allowed before everyone has voted; auto-reveal still
	// happens when all active players have voted (see CastVote).
	canReveal := !r.revealed
	return RoomSnapshot{
		Room:      r.slug,
		Cards:     append([]string(nil), r.cards...),
		Revealed:  r.revealed,
		CanReveal: canReveal,
		Players:   players,
	}
}

func (r *Room) publishLocked() {
	if r.events == nil {
		return
	}
	snap := r.snapshotLocked()
	ch := r.events
	// Do not drop snapshots: a non-blocking send caused missed "player left" updates for
	// everyone. Send without holding the room lock (snapshot is already a copy).
	go func() {
		ch <- snap
	}()
}

func (r *Room) UpsertPlayer(uuid, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p, ok := r.players[uuid]; ok {
		p.Name = name
	} else {
		r.players[uuid] = &Player{
			UUID:      uuid,
			Name:      name,
			Vote:      nil,
			Connected: false,
			seqCreated: r.nextSeq,
		}
		r.nextSeq++
	}
	r.publishLocked()
}

func (r *Room) SetConnected(uuid string, connected bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.players[uuid]
	if p == nil {
		return
	}

	if connected {
		p.Connected = true
		p.disconnAt = time.Time{}
		r.publishLocked()
		return
	}

	if p.Connected {
		p.Connected = false
		p.disconnAt = time.Now()
		r.publishLocked()

		disconnAt := p.disconnAt
		go r.scheduleRemoval(uuid, disconnAt)
	}
}

func (r *Room) scheduleRemoval(uuid string, disconnAt time.Time) {
	time.Sleep(r.gracePeriod)

	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.players[uuid]
	if p == nil {
		return
	}
	if p.Connected {
		return // reconnected
	}
	if !p.disconnAt.Equal(disconnAt) {
		return // another disconnect cycle
	}

	delete(r.players, uuid)
	// If reveal hasn't happened yet, removing a player may enable reveal.
	if !r.revealed && r.allActiveVotedLocked() {
		r.revealed = true
	}
	r.publishLocked()
}

func (r *Room) activePlayerVoteRequirementLocked(p *Player) bool {
	// A player counts as "active" if they are currently connected, or if they have already voted
	// (vote is kept during the grace period even if they disconnect).
	return p.Connected || p.Vote != nil
}

func (r *Room) allActiveVotedLocked() bool {
	for _, p := range r.players {
		if !r.activePlayerVoteRequirementLocked(p) {
			continue
		}
		if p.Vote == nil {
			return false
		}
	}
	// If there are no active players, don't reveal.
	for _, p := range r.players {
		if r.activePlayerVoteRequirementLocked(p) {
			return true
		}
	}
	return false
}

func (r *Room) CastVote(uuid, card string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.revealed {
		return errors.New("voting already revealed")
	}

	p := r.players[uuid]
	if p == nil {
		return errors.New("unknown player")
	}

	// Ensure card is in current list.
	ok := false
	for _, c := range r.cards {
		if c == card {
			ok = true
			break
		}
	}
	if !ok {
		return errors.New("invalid card")
	}

	p.Vote = &card

	if r.allActiveVotedLocked() {
		r.revealed = true
	}
	r.publishLocked()
	return nil
}

func (r *Room) UpdatePlayerName(uuid, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.players[uuid]
	if p == nil {
		return errors.New("unknown player")
	}
	p.Name = name
	r.publishLocked()
	return nil
}

func (r *Room) UpdateCards(cards []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cards = append([]string(nil), cards...)
	r.revealed = false
	for _, p := range r.players {
		p.Vote = nil
	}
	r.publishLocked()
}

func (r *Room) StartNewVoting() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.revealed == false && r.allActiveVotedLocked() {
		// No-op; vote set already implies reveal will happen when last vote arrives.
	}
	r.revealed = false
	for _, p := range r.players {
		p.Vote = nil
	}
	r.publishLocked()
	return nil
}

func (r *Room) RevealNow() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.revealed {
		return nil
	}
	r.revealed = true
	r.publishLocked()
	return nil
}


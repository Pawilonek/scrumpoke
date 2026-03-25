package game

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

func RandomUUID() string {
	// Generate UUIDv4 with RFC4122 bits.
	var b [16]byte
	_, _ = rand.Read(b[:])
	// Version 4 (0100)
	b[6] = (b[6] & 0x0f) | 0x40
	// Variant 10xx
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		b[0], b[1], b[2], b[3],
		b[4], b[5],
		b[6], b[7],
		b[8], b[9],
		b[10], b[11], b[12], b[13], b[14], b[15],
	)
}

func randomFromCharset(n int, charset string) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	max := byte(len(charset) - 1)
	for i := 0; i < n; i++ {
		var b [1]byte
		_, _ = rand.Read(b[:])
		out[i] = charset[int(b[0])%(int(max)+1)]
	}
	return string(out)
}

func RandomRoomSlug() string {
	// Matches [a-z0-9-] and is URL-friendly.
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	return strings.ToLower(
		fmt.Sprintf("%s-%s", randomFromCharset(6, chars), randomFromCharset(4, chars)),
	)
}

func RandomPlayerName() string {
	// Allowed chars are letters/digits/spaces only.
	// Keep it short to satisfy backend limits.
	adjs := []string{"Scrum", "Poke", "Swift", "Brave", "Calm", "Turbo", "Quick", "Dapper"}

	seed := time.Now().UnixNano()
	a := adjs[int(seed%int64(len(adjs)))]
	// Add randomness so names don't repeat too much.
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	rnd := binary.LittleEndian.Uint32(buf[:4]) % 10000
	return fmt.Sprintf("%s %d", a, rnd)
}


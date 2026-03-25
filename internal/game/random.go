package game

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
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

func randIntn(n int) int {
	if n <= 0 {
		return 0
	}
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return int(binary.LittleEndian.Uint64(buf[:]) % uint64(n))
}

func RandomRoomSlug() string {
	const maxTries = 128
	for range maxTries {
		a := embeddedAdjectives[randIntn(len(embeddedAdjectives))]
		noun := embeddedNouns[randIntn(len(embeddedNouns))]
		adjPart := slugifyWord(a)
		nounPart := slugifyWord(noun)
		if adjPart == "" || nounPart == "" {
			continue
		}
		slug := adjPart + "-" + nounPart
		if err := ValidateRoomSlug(slug); err == nil {
			return slug
		}
	}
	// Fallback: alphanumeric slug (always valid).
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	return strings.ToLower(
		fmt.Sprintf("%s-%s", randomFromCharset(6, chars), randomFromCharset(4, chars)),
	)
}

func RandomPlayerName() string {
	const maxTries = 128
	for range maxTries {
		noun := embeddedNouns[randIntn(len(embeddedNouns))]
		raw := normalizeNounForPlayerName(noun)
		if raw == "" {
			continue
		}
		name := titlePlayerName(raw)
		if _, err := ValidatePlayerName(name); err == nil {
			return name
		}
	}
	// Fallback if every pick normalizes badly.
	const chars = "abcdefghijklmnopqrstuvwxyz"
	return fmt.Sprintf("Player %s", randomFromCharset(6, chars))
}

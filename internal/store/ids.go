package store

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// NewUUID generates a random RFC-4122 v4 UUID (generated in Go, not the DB, for
// cross-engine portability).
func NewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("store: crypto/rand failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	var dst [36]byte
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])
	return string(dst[:])
}

const nanoAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// NewNanoID returns a URL-safe random id of length n (used for shareable group
// public ids). Uses rejection sampling for an unbiased distribution.
func NewNanoID(n int) string {
	out := make([]byte, n)
	buf := make([]byte, n)
	i := 0
	for i < n {
		if _, err := rand.Read(buf); err != nil {
			panic("store: crypto/rand failed: " + err.Error())
		}
		for _, c := range buf {
			if int(c) < 62*4 { // 248: largest multiple of 62 below 256 avoids bias
				out[i] = nanoAlphabet[int(c)%62]
				i++
				if i == n {
					break
				}
			}
		}
	}
	return string(out)
}

// nowISO returns the current UTC time formatted as the canonical ISO-8601
// string used throughout the schema.
func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// FromNow returns an ISO-8601 UTC timestamp d in the future.
func FromNow(d time.Duration) string {
	return time.Now().UTC().Add(d).Format("2006-01-02T15:04:05.000Z")
}

package eveses

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// VerifyWebhook verifies an Eveses webhook signature.
//
// Eveses signs every webhook delivery with HMAC-SHA256 over
// "{timestamp}.{body}" using the endpoint's signing secret. Two headers
// carry the proof:
//
//	X-Eveses-Signature  -> "sha256=<hex>"
//	X-Eveses-Timestamp  -> unix seconds (string)
//
// Always pass the raw request body bytes — round-tripping through
// json.Unmarshal/json.Marshal reorders keys and breaks the signature.
//
// tolerance bounds how far the timestamp may drift from "now". Pass 0 to
// disable the staleness check. The default in other SDKs is 300s.
//
// Returns (true, nil) iff the signature is valid and within tolerance.
// (false, nil) is returned for any verification failure (bad signature,
// expired timestamp, missing inputs); error is reserved for genuine
// programming/library faults (currently never returned, but the signature
// allows future expansion).
func VerifyWebhook(rawBody []byte, sigHeader, tsHeader, secret string, tolerance time.Duration) (bool, error) {
	if sigHeader == "" || secret == "" {
		return false, nil
	}

	expectedHex := stripSigPrefix(sigHeader)
	if expectedHex == "" || !isHex(expectedHex) {
		return false, nil
	}

	if tsHeader == "" {
		return false, nil
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(tsHeader), 10, 64)
	if err != nil || ts <= 0 {
		return false, nil
	}

	if tolerance > 0 {
		now := time.Now().Unix()
		drift := now - ts
		if drift < 0 {
			drift = -drift
		}
		if time.Duration(drift)*time.Second > tolerance {
			return false, nil
		}
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	computed := mac.Sum(nil)

	expected, err := hex.DecodeString(expectedHex)
	if err != nil {
		return false, nil
	}
	if len(expected) != len(computed) {
		return false, nil
	}
	return hmac.Equal(computed, expected), nil
}

func stripSigPrefix(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "sha256=") {
		return trimmed[len("sha256="):]
	}
	return trimmed
}

func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

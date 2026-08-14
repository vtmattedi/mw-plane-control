package identitystrong

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	totpDigits = 6
	totpPeriod = int64(30)
)

func ValidateTOTP(secret []byte, code string, now time.Time, lastAcceptedStep int64) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return 0, false
	}
	if _, err := strconv.Atoi(code); err != nil {
		return 0, false
	}

	current := now.UTC().Unix() / totpPeriod
	for _, delta := range []int64{-1, 0, 1} {
		step := current + delta
		if step < 0 || step <= lastAcceptedStep {
			continue
		}
		expected := totpAtStep(secret, step)
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

func totpAtStep(secret []byte, step int64) string {
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binaryCode := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	value := binaryCode % 1000000
	return fmt.Sprintf("%06d", value)
}

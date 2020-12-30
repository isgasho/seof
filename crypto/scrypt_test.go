package crypto

import (
	"golang.org/x/crypto/scrypt"
	"testing"
	"time"
)

func TestScryptParameters(t *testing.T) {
	keyLen := 32 * 3
	password := []byte("some password")
	salt := RandBytes(keyLen)
	start := time.Now()
	count := 3
	for i := 0; i < count; i++ {
		_, err := scrypt.Key(password, salt,
			int(CurrentSCryptParameters.N), int(CurrentSCryptParameters.R), int(CurrentSCryptParameters.P), keyLen)
		if err != nil {
			t.Error(err)
		}
	}
	duration := time.Now().Sub(start)
	scryptMs := duration.Milliseconds() / int64(count)
	//fmt.Println("Scrypt parameters taking on average in this CPU:", scryptMs, "ms")
	if scryptMs < 250 {
		t.Errorf("Scrypt should take at least 250ms, it took %vms -- Time to increase its parameters", scryptMs)
	}
}
package bmdtest

import (
	"math/rand"

	"github.com/monetas/bmutil/wire"
)

// RandomShaHash returns a ShaHash with a random string of bytes in it.
func RandomShaHash() *wire.ShaHash {
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		b[i] = byte(rand.Intn(256))
	}
	hash, _ := wire.NewShaHash(b)
	return hash
}

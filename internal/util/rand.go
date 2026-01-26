package util

import (
	"hash/fnv"
	"math/rand"
)

func NewSeededRand(seedInputs ...string) *rand.Rand {
	hasher := fnv.New64a()
	for _, input := range seedInputs {
		_, _ = hasher.Write([]byte(input))
	}
	seed := int64(hasher.Sum64())
	return rand.New(rand.NewSource(seed))
}

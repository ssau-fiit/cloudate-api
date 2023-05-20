package util

import "math/rand"

func GetRandomNumber() int {
	min := 111111
	max := 999999
	return rand.Intn(max-min) + min
}

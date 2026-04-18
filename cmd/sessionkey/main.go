package main

import (
	"encoding/base64"

	"github.com/gorilla/securecookie"
)

func main() {
	randomKey := securecookie.GenerateRandomKey(32)
	base64Key := base64.StdEncoding.EncodeToString(randomKey)
	println("Generated Session Key (base64):", base64Key)
}

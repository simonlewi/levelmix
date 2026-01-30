//go:build !ee

package main

import "log"

func run() {
	log.Fatal("Built without enterprise features. Rebuild with: go build -tags ee")
}

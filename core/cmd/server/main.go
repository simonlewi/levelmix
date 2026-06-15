package main

// GitCommit is set via ldflags during build: -X main.GitCommit=$(git rev-parse --short HEAD)
var GitCommit = "dev"

func main() {
	run()
}

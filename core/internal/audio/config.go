package audio

import "os"

var (
	// debugMode is set from DEBUG environment variable
	debugMode = os.Getenv("DEBUG") == "true"
)

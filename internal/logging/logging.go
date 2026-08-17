package logging

import "log"

// Setup applies a simple log format; expand with levels if needed.
func Setup(debug bool) {
	flags := log.LstdFlags
	if debug {
		flags |= log.Lshortfile
	}
	log.SetFlags(flags)
}

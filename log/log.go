package log

import "fmt"

var debug bool

func EnableDebug() {
	debug = true
}
func Debug(message string) {
	if debug {
		fmt.Print(message)
	}
}

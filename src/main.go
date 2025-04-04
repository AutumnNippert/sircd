package main

import (
	"fmt"

	. "sircd/src/util"
	. "sircd/src/net"
)

func Init() {
	// Load configuration
	InitConfig()

	fmt.Println("Starting IRC server on", HOST+":"+PORT)
	fmt.Println("Version:", VERSION)

	// Initialize logging
	InitLogging()
}

func main() {
	Init()
	// Start listening for connections
	Listen(HOST, PORT)
}

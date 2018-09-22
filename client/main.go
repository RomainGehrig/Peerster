package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var Usage = func() {
		fmt.Printf("Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	var uiPort = flag.String("UIPort", "8080", "port for the UI client")
	var msg = flag.String("msg", "", "message to be sent")

	Usage()
	flag.Parse()

	fmt.Printf("Given arguments where: %s, %s", *uiPort, *msg)

}

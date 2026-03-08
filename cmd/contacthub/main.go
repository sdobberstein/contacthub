package main

import (
	"fmt"
	"os"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("contacthub %s\n", version)
		os.Exit(0)
	}

	fmt.Println("ContactHub - not yet implemented")
	os.Exit(0)
}

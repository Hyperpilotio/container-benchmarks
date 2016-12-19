package main

import (
	"fmt"
)

// We use this to occupy all the cpu shares given to this container.
// Not looking to introduce any other resource interference.
func main() {
	fmt.Println("Starting to spin....")
	for {
	}
}

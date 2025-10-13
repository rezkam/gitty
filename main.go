package main

import (
	"fmt"

	"github.com/rezkam/gritty/cmd"
	_ "github.com/rezkam/gritty/provider"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

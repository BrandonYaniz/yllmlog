package main

import (
	"fmt"
	"os"

	"github.com/BrandonYaniz/yllmlog/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version.Current())
		return
	}

	fmt.Println("yllmlog CLI skeleton")
	fmt.Println("Run `yllmlog version` to print the current version.")
}

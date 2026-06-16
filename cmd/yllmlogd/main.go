package main

import (
	"fmt"

	"github.com/BrandonYaniz/yllmlog/internal/version"
)

func main() {
	fmt.Printf("yllmlogd skeleton %s\n", version.Current())
}

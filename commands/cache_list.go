package commands

import "fmt"

type CacheListCmd struct {
	Remote string `arg:"--remote"`
}

func CacheListCommand() {
	fmt.Println("1TB in cache!")
}

package main

import "fmt"

func logBanner(title string) {
	fmt.Println("=======================================")
	fmt.Println(title)
	fmt.Println("=======================================")
}

func logStep(msg string) {
	fmt.Println("-> " + msg)
}

func logOK(msg string) {
	fmt.Println("[OK] " + msg)
}

func logFail(msg string) {
	fmt.Println("[FAIL] " + msg)
}

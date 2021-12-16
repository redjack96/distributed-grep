package util

import (
	"fmt"
	"os"
)

func PanicOn(e error) {
	if e != nil {
		fmt.Println(e.Error())
		os.Exit(1)
	}
}

func PanicIf(cond bool, msg string) {
	if cond {
		fmt.Println(msg)
		os.Exit(1)
	}
}

func PanicIfMessage(e error, s string) {
	if e != nil {
		fmt.Println(s, ":", e.Error())
		os.Exit(1)
	}
}

func Error(e error) {
	if e != nil {
		fmt.Println(e.Error())
	}
}

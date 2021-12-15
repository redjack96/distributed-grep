package util

import (
	"fmt"
	"os"
)

func PanicIf(e error) {
	if e != nil {
		fmt.Println(e.Error())
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

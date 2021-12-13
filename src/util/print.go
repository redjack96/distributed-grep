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

func Error(e error) {
	if e != nil {
		fmt.Println(e.Error())
	}
}

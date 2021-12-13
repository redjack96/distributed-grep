package main

import (
	"fmt"
	mapreduce "github.com/redjack/distributed-grep/src/map-reduce"
	"github.com/redjack/distributed-grep/src/util"
	"os"
)

// Esempio di utilizzo del grep distribuito
// Esegue una chiamata a procedura remota (sulla macchina locale)
// Per lanciare via linea di comando, entra nella cartella main ed esegui "go run ."
func main() {
	var argv = os.Args // il nome dell'eseguibile è l'elemento 0
	fmt.Println(os.Getwd())
	switch len(argv) {
	case 3:
		startGrep(argv[1], argv[2]) // Come in C, il primo argomento sta a indice 1
	default:
		fmt.Println("Utilizzo:\nprog.exe file-path regex") // ci sarebbe anche println(), ma potrebbe essere rimossa nelle versioni successive
	}
}

func startGrep(file string, regex string) {
	var data, err = os.ReadFile(file) // Leggo l'intero file
	util.PanicIf(err)
	mapreduce.StartMaster(string(data), regex)
}

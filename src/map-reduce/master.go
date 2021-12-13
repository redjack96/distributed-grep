package map_reduce

import (
	"fmt"
	"strings"
)

const (
	address = "127.0.0.1"
	port    = 1234
)

// Task una sorta di lambda
type Task func() []string

// TODO: usare gRPC
func StartMaster(data, grep string) {
	fmt.Printf("Master started at %s:%d\n", address, port)

	var channel = make(chan int)

	for i := 0; i < MaxWorkers; i++ {
		go StartWorker(i, channel) // goroutine
	}

	// se il main finisce prima delle goroutine, non stamperanno nulla
	for i := 0; i < MaxWorkers; i++ {
		<-channel // aspetto il worker
	}

	// grep semplice
	lines := strings.Split(data, "\n")
	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], grep) {
			fmt.Printf("%d: %s\n", i, lines[i])
		}
	}

	fmt.Printf("Master exited\n")
}

func _map() {

}
func _reduce() {

}

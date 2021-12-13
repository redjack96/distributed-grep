package map_reduce

import (
	"fmt"
)

// le costanti con la lettera maiuscola sono pubbliche
const (
	Address    = "127.0.0.1"
	MaxWorkers = 10
	BasePort   = 8000
)

func StartWorker(id int, channel chan int) {
	fmt.Printf("Worker %d started at %s:%d\n", id, Address, BasePort+(id%MaxWorkers))
	fmt.Printf("Worker %d exited\n", id)
	channel <- id // ho finito!
}

package map_reduce

import (
	"context"
	"fmt"
	pb "github.com/redjack/distributed_grep/proto"
	"github.com/redjack/distributed_grep/src/util"
	"google.golang.org/grpc"
	"runtime"
	"strings"
	"time"
)

type masterMetadata struct {
	channel    chan int32
	connection *grpc.ClientConn
	id         int32
	port       int32
	client     pb.DistGrepClient
	ctx        context.Context
	cancel     context.CancelFunc
}

func Init() []masterMetadata {
	// Lo slice dei metadati per i worker
	var metadataSlice []masterMetadata
	for id := int32(0); id < MaxWorkers; id++ {
		var channel = make(chan int32)
		go RegisterWorker(id, channel) // go-routine per registrare il grepServer
		<-channel                      // aspetto che il worker sia in listening
		port := BasePort + id
		fmt.Printf("Master: Inizializzo la connessione col worker %d alla porta %d\n", id, port)
		clientConn, err := grpc.Dial(fmt.Sprintf("localhost:%d", port), grpc.WithInsecure()) // CHIAMATA ASINCRONA (senza TLS)
		util.PanicIf(err)

		fmt.Printf("Master: Creo il DistGrepClient\n")
		client := pb.NewDistGrepClient(clientConn)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		metadataSlice = append(metadataSlice, masterMetadata{
			channel:    channel,
			port:       BasePort + id,
			ctx:        ctx,
			connection: clientConn,
			client:     client,
			cancel:     cancel,
			id:         id,
		})
	}
	return metadataSlice
}

// StartMaster deve fare da server per chi chiama il servizio
// inoltre deve fare da client verso i worker, perché deve assegnare loro un task map() o reduce()
func StartMaster(file, regex string) {
	// TODO: il master deve essere chiamato con RPC. per ora comunica solo lui con workers.
	// TODO: incrementare il numero di worker in base alla dimensione del file oppure creare tante GO-routine quanti sono i core disponibili.

	lines := strings.Split(file, "\n")
	numLines := len(lines)
	linesPerChunk := int32(numLines) / MaxWorkers
	fmt.Println("lines: ", lines, "lines per chunk: ", linesPerChunk)

	var metadataSlice = Init()

	var result string
	var resultSize int32
	for i := range metadataSlice {
		// TODO: la Map deve accettare una funzione...
		fileChunk := strings.Join(lines[int32(i)*linesPerChunk:int32(i+1)*linesPerChunk-1], "\n")
		chunkResult := Map(fileChunk, regex, metadataSlice[i])
		resultSize += int32(strings.Count(chunkResult, "\n"))
		result += chunkResult
	}

	fmt.Printf("Found matches: %d\n%s", resultSize, result)

	for i := range metadataSlice {
		_ = metadataSlice[i].connection.Close()
	}

	// TODO solo per testare cosa succede alla fine. Togli dopo
	for {
		var input string
		fmt.Println("Master: sto per uscire... goroutine rimaste attive: ", runtime.NumGoroutine(), "Digita 'exit' per uscire")
		_, err := fmt.Scanln(&input)
		util.PanicIf(err)
		if input == "exit" {
			return
		}
	}
}

// Map restituisce un risultato intermedio nella porzione di file con le righe che contengono regex se presenti
func Map(fileChunk string, regex string, mm masterMetadata) string {
	grepTask := pb.GrepRequest{
		FilePortion: fileChunk,
		Regex:       regex,
		WorkerId:    mm.id,
	}
	// 1 secondo di timeout
	defer mm.cancel() // annulla il timeout al termine dalla main()
	fmt.Printf("Master: assegno map al worker %d\n", mm.id)
	grepResult, err := mm.client.StartGrep(mm.ctx, &grepTask) // ---> INVOCA IL METODO REMOTO (vedi worker)
	util.PanicIf(err)
	if grepResult.RowNumber == 0 {
		return ""
	}
	return grepResult.Rows
}

func Reduce() {
	// TODO
}
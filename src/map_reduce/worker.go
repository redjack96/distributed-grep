package map_reduce

import (
	"context"
	"fmt"
	pb "github.com/redjack/distributed_grep/proto"
	"github.com/redjack/distributed_grep/src/util"
	"google.golang.org/grpc"
	"net"
	"strings"
)

// le costanti con la lettera maiuscola sono pubbliche
const (
	Address          = "127.0.0.1"
	MaxWorkers int32 = 10
	BasePort   int32 = 8000
)

// grepServer is used to implement proto.DistGrepServer.
type grepServer struct {
	pb.DistGrepServer // questa struct è necessaria per retrocompatibilità. Campo anonimo: si può accedere con <variabile grepServer>.UnimplementedDistGrepServer
}

// StartGrep Esegue la grep su una porzione di file
func (s *grepServer) StartGrep(ctx context.Context, in *pb.GrepRequest) (*pb.GrepResult, error) {
	fmt.Printf("Sto cercando: %v\n", in.Regex) //  // %v permette di stampare solo i valori di una struct

	// grep semplice, ma su una porzione del file.
	result := ""
	lines := strings.Split(in.FilePortion, "\n")
	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], in.Regex) {
			result += fmt.Sprintf("%s\n", lines[i])
		}
	}
	rowFound := int64(strings.Count(result, "\n") + 1)
	fmt.Printf("Worker %d Fine della ricerca - righe: %d\n", in.WorkerId, rowFound)
	return &pb.GrepResult{
		Rows:      result,
		RowNumber: rowFound,
	}, nil
}

// RegisterWorker deve fare da grepServer a cui richiedere di svolgere la map(), ovvero una grep
// Al termine restituirà la risposta al master.
// NOTA: non vuoi che i workers siano processi a parte! Meglio spawnare delle goroutine.
func RegisterWorker(id int32, channel chan int32) {

	doGrpcServer(id, channel)

	fmt.Printf("Worker %d exited\n", id)
}

func doGrpcServer(id int32, channel chan int32) {
	port := BasePort + (id % MaxWorkers)
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	util.PanicIfMessage(err, fmt.Sprintf("Worker %d Fallimento Listen()", id))
	s := grpc.NewServer()
	pb.RegisterDistGrepServer(s, &grepServer{}) // registra i servizi del server

	fmt.Printf("Worker listening at %v\n", lis.Addr())
	channel <- id // ho finito!
	// fmt.Printf("Worker %d started at %s:%d\n", id, Address, port)
	// Servo eventuali client.
	err = s.Serve(lis)
	util.PanicIfMessage(err, fmt.Sprintf("Worker %d: Fallimento Serve()", id))
	fmt.Printf("Worker %d", id)
}

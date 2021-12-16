package map_reduce

import (
	"context"
	"fmt"
	pb "github.com/redjack/distributed_grep/proto"
	"github.com/redjack/distributed_grep/util"
	"google.golang.org/grpc"
	"net"
	"strings"
)

// grepServer is used to implement proto.DistGrepServer.
type taskServer struct {
	pb.GrepTaskServer // questa struct è necessaria per retrocompatibilità. Campo anonimo: si può accedere con <variabile taskServer>.UnimplementedDisttaskServer
}

// Map - funzione che esegue una map grep su una porzione di file
func (s *taskServer) Map(ctx context.Context, in *pb.GrepInput) (*pb.GrepOutput, error) {
	fmt.Printf("Sto cercando: %v\n", in.Regex) // %v permette di stampare solo i valori di una struct

	// grep semplice, ma su una porzione del file.
	result := ""
	lines := strings.Split(in.FilePortion, "\n")
	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], in.Regex) {
			result += fmt.Sprintf("%s\n", lines[i]) // salvo le righe con un match in una stringa unica
		}
	}
	rowFound := int64(strings.Count(result, "\n") + 1)
	fmt.Printf("Worker %d Fine della ricerca - righe: %d\n", in.WorkerId, rowFound)
	return &pb.GrepOutput{
		Rows:      result,
		RowNumber: rowFound,
	}, nil
}

// RegisterWorker deve fare da taskServer a cui richiedere di svolgere la map(), ovvero una grep
// Al termine restituirà la risposta al master.
func RegisterWorker(id int, channel chan int32) {
	conf := util.GetConfig()

	port := conf.BaseClientPort + (id % conf.MaxWorkers)
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	util.PanicIfMessage(err, fmt.Sprintf("Worker %d Fallimento Listen()", id))
	s := grpc.NewServer()
	pb.RegisterGrepTaskServer(s, &taskServer{}) // registra i servizi del server

	fmt.Printf("Worker listening at %v\n", lis.Addr())

	// Servo le richiesta del master -.-
	err = s.Serve(lis)
	util.PanicIfMessage(err, fmt.Sprintf("Worker %d: Fallimento Serve()", id))
	fmt.Printf("Worker %d", id)

	fmt.Printf("Worker %d exited\n", id)
}

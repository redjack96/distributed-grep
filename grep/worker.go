package main

import (
	"context"
	"fmt"
	pb "github.com/redjack/distributed_grep/proto"
	"github.com/redjack/distributed_grep/util"
	"google.golang.org/grpc"
	"net"
	"strings"
)

// grepServer is used to implement proto.DistGrepServer. Rappresenta un worker.
type taskServer struct {
	pb.GrepTaskServer     // questa struct è necessaria per retrocompatibilità. Campo anonimo: si può accedere con <variabile taskServer>.UnimplementedDisttaskServer
	id                int // l' id del worker
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

func main() {
	lis, id := tryListen()
	util.PanicIf(id == -1, "Worker - Impossibile trovare una porta libera. Esco.")

	s := grpc.NewServer()
	pb.RegisterGrepTaskServer(s, &taskServer{id: id}) // registra i servizi del server

	fmt.Printf("Worker %d listening at %v\n", id, lis.Addr())

	// Servo le richiesta del master -.-
	err := s.Serve(lis)
	util.PanicIfMessage(err, fmt.Sprintf("Worker %d: Fallimento Serve()", id))

	fmt.Printf("Worker %d exited\n", id)
}

func tryListen() (net.Listener, int) {
	var lis net.Listener
	var err error

	conf := util.GetConfig()

	for port := conf.BaseClientPort; port < conf.BaseClientPort+conf.MaxWorkers; port++ {
		lis, err = net.Listen("tcp", fmt.Sprintf("%s:%d", conf.Address, port))
		if err == nil {
			return lis, port - conf.BaseClientPort
		}
	}

	fmt.Printf("Impossibile trovare una porta aperta nel range %d-%d\n",
		conf.BaseClientPort, conf.BaseClientPort+conf.MaxWorkers-1)
	return lis, -1
}

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
	pb.GrepTaskServer       // Necessaria per retrocompatibilità. Campo anonimo: si può accedere con <variabile taskServer>.UnimplementedDisttaskServer
	id                int32 // l' id del worker
	conf              util.Configuration
	lis               net.Listener
}

// tryListen permette al taskServer di provare a mettersi in modalità listening su una porta
// nel range [BaseWorkerPort, BaseWorkerPort+MaxWorkers] e imposta i campi della struct t di conseguenza.
func (t *taskServer) tryListen() {
	var err error
	basePort := t.conf.BaseWorkerPort
	for port := basePort; port < basePort+t.conf.MaxWorkers; port++ {
		t.lis, err = net.Listen("tcp", fmt.Sprintf("%s:%d", t.conf.Address, port))
		if err == nil {
			t.id = int32(port - basePort)
			return
		}
	}

	fmt.Printf("Worker - Impossibile trovare una porta libera nel range %d-%d\n",
		basePort, basePort+t.conf.MaxWorkers-1)
	t.id = -1
	return
}

// Map - funzione che esegue una map grep su una porzione di file
func (t *taskServer) Map(ctx context.Context, in *pb.GrepInput) (*pb.GrepOutput, error) {
	fmt.Printf("Sto cercando: %v\n", in.Regex) // %v permette di stampare solo i valori di una struct

	// grep semplice, ma su una porzione del file.
	result := ""
	lines := strings.Split(in.FilePortion, "\n")
	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], in.Regex) {
			result += fmt.Sprintf("%s\n", lines[i]) // salvo le righe con un match in una stringa unica
		}
	}

	rowFound := int64(strings.Count(result, "\n"))
	fmt.Printf("Worker %d Fine della ricerca - righe: %d\n", in.WorkerId, rowFound)
	return &pb.GrepOutput{
		Rows:      result,
		RowNumber: rowFound,
	}, nil
}

// Reduce - restituisce semplicemente l' input
func (t *taskServer) Reduce(ctx context.Context, in *pb.GrepInput) (*pb.GrepOutput, error) {
	fmt.Printf("Worker %d: Ridotto l'output in %d righe \n", t.id, strings.Count(in.FilePortion, "\n")) // %v permette di stampare solo i valori di una struct
	return &pb.GrepOutput{
		Rows:      in.FilePortion,
		RowNumber: int64(strings.Count(in.FilePortion, "\n")),
	}, nil
}

func main() {
	t := taskServer{
		conf: *util.GetConfig(),
	}
	t.tryListen()
	util.PanicIf(t.id == -1, "Worker - Esco.")

	s := grpc.NewServer()
	pb.RegisterGrepTaskServer(s, &t) // registra i servizi del server

	fmt.Printf("Worker %d listening at %v\n", t.id, t.lis.Addr())

	// Servo le richiesta del master -.-
	err := s.Serve(t.lis)
	util.PanicIfMessage(err, fmt.Sprintf("Worker %d: Fallimento Serve()", t.id))

	fmt.Printf("Worker %d exited\n", t.id)
}

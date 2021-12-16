package main

import (
	"context"
	"fmt"
	pb "github.com/redjack/distributed_grep/proto"
	"github.com/redjack/distributed_grep/util"
	"google.golang.org/grpc"
	"net"
	"os"
	"strings"
	"time"
)

// taskMaster implement proto.MapReduceGrepClient. Permette la comunicazione RPC con i worker. Usata nel service RPC
type taskMaster struct {
	pb.GrepTaskClient // Questa struct è necessaria per retro-compatibilità. Campo anonimo: si può accedere con <variabile grepMaster>.UnimplementedDistGrepServer
}

// serverMaster implement proto.MasterGrepServer. Permette la comunicazione RPC con un client. Usata nella main
type serverMaster struct {
	pb.GoGrepServer
}

var conf = util.GetConfig()

// DistributedGrep - funzione che risponde a un client
func (s *serverMaster) DistributedGrep(ctx context.Context, in *pb.GrepRequest) (*pb.GrepResult, error) {

	fmt.Printf("Avviata grep sul file %s per cercare '%s'\n", in.FilePath, in.Regex) // %v permette di stampare solo i valori di una struct

	mapConnectionTask := make(map[*grpc.ClientConn]*pb.GrepTaskClient)

	connectToWorkers(mapConnectionTask)
	fmt.Println("Numero di workers individuati: ", len(mapConnectionTask))

	fileContent, err := os.ReadFile(in.FilePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(fileContent), "\n")
	numLines := len(lines)
	linesPerChunk := numLines / conf.MaxWorkers
	fmt.Printf("lines: %d lines per chunk: %d\n", len(lines), linesPerChunk)
	// assegno un chunk per worker

	var mapOutput []*pb.GrepOutput
	i := 0
	for _, client := range mapConnectionTask {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		var partialOutput *pb.GrepOutput
		var err error
		if client != nil {
			fileChunk := strings.Join(lines[i*linesPerChunk:(i+1)*linesPerChunk-1], "\n")
			partialOutput, err = (*client).Map(ctx, &pb.GrepInput{
				FilePortion: fileChunk,
				Regex:       in.Regex,
				WorkerId:    int32(i), // TODO: non è proprio corretto, andrebbe messo porta - 8000
			})
			util.PanicOn(err)
		}
		cancel()
		mapOutput = append(mapOutput, partialOutput)
		i++
	}

	//shuffleOutput := ShuffleAndSort(mapOutput)
	//ReduceGrep(shuffleOutput, workers)
	var grepped = make([]string, 0, conf.MaxWorkers)
	for _, output := range mapOutput {
		if output != nil {
			grepped = append(grepped, output.Rows)
		}
	}
	finalResult := strings.Join(grepped, "\n")

	// TODO:
	//  In caso di errore ritorna qualcosa del genere:
	// if false {
	// 	return &pb.GrepResult{}, status.Error(codes.Unavailable, "Error message")
	// }

	closeConnection(mapConnectionTask)

	return &pb.GrepResult{
		Rows: finalResult,
	}, nil
}

func connectToWorkers(mapConnectionTask map[*grpc.ClientConn]*pb.GrepTaskClient) {

	// Creo un client per ogni connessione/worker disponibile

	for port := conf.BaseClientPort; port < conf.BaseClientPort+conf.MaxWorkers; port++ {
		clientConn, err := grpc.Dial(fmt.Sprintf("%s:%d", conf.Address, port), grpc.WithInsecure(), grpc.WithBlock()) // CHIAMATA ASINCRONA (senza TLS)
		util.PanicOn(err)
		if clientConn != nil {
			client := pb.NewGrepTaskClient(clientConn)
			mapConnectionTask[clientConn] = &client
		}
	}

	fmt.Printf("Connesso con %d workers", len(mapConnectionTask))
}

func ReduceGrep(clientConnections []*pb.GrepTaskClient) []*pb.GrepOutput {

	var mapOutput []*pb.GrepOutput
	for _, client := range clientConnections {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)

		input := pb.GrepInput{
			FilePortion: "",
			Regex:       "",
			WorkerId:    0,
		}

		partialOutput, err := (*client).Reduce(ctx, &input)
		util.PanicOn(err)

		mapOutput = append(mapOutput, partialOutput)
		cancel()
	}

	return mapOutput
}

func closeConnection(clientMap map[*grpc.ClientConn]*pb.GrepTaskClient) {
	for connection, _ := range clientMap {
		if connection != nil {
			_ = (*connection).Close()
		}
	}
}

func main() {
	// Prendo i valori dell' indirizzo e della porta da un file di configurazione
	conf := util.GetConfig()
	addr := conf.Address
	port := conf.MasterPort

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", addr, port))
	util.PanicIfMessage(err, "Master: error in Listen()")

	serv := grpc.NewServer()
	pb.RegisterGoGrepServer(serv, &serverMaster{}) // registra i servizi disponibili nel server (vedi file master.proto)

	fmt.Printf("Master server listening at %v\n", lis.Addr())

	// Servo eventuali client.
	err = serv.Serve(lis)
	util.PanicIfMessage(err, fmt.Sprintf("Master: error in Serve()"))
}

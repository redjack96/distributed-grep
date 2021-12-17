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

// serverMaster implement proto.MasterGrepServer. Permette la comunicazione RPC con un client. Usata nella main
type serverMaster struct {
	pb.GoGrepServer
}

var Conf = util.GetConfig()
var mapConnectionTask = make(map[*grpc.ClientConn]*pb.GrepTaskClient)

// DistributedGrep - funzione che risponde a un client
func (s *serverMaster) DistributedGrep(ctx context.Context, in *pb.GrepRequest) (*pb.GrepResult, error) {

	fmt.Printf("Avviata grep sul file %s per cercare '%s'\n", in.FilePath, in.Regex) // %v permette di stampare solo i valori di una struct

	fileContent, err := os.ReadFile(in.FilePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(fileContent), "\n")
	numLines := len(lines)
	linesPerChunk := numLines / len(mapConnectionTask) // divido per il numero di worker
	fmt.Printf("lines: %d lines per chunk: %d\n", len(lines), linesPerChunk)
	// assegno un chunk per worker

	outChannels := make([]chan *pb.GrepOutput, 0, Conf.MaxWorkers)

	// MAP TASK
	i := 0
	for _, client := range mapConnectionTask {
		fileChunk := strings.Join(lines[i*linesPerChunk:(i+1)*linesPerChunk-1], "\n")
		canaleOutput := make(chan *pb.GrepOutput)
		go Map(ctx, canaleOutput, client, &pb.GrepInput{
			FilePortion: fileChunk,
			Regex:       in.Regex,
			WorkerId:    int32(i),
		})
		outChannels = append(outChannels, canaleOutput)
		i++
	}

	// Barriera di sincronizzazione per MAP
	var grepped = make([]string, 0, Conf.MaxWorkers)
	for _, ch := range outChannels {
		out := <-ch
		if out != nil {
			grepped = append(grepped, out.Rows)
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

func Map(ctx context.Context, canale chan *pb.GrepOutput, client *pb.GrepTaskClient, in *pb.GrepInput) {

	partialOutput, err := (*client).Map(ctx, in)
	if err != nil {
		canale <- &pb.GrepOutput{}
		return
	}
	canale <- partialOutput
}

func connectToWorkers(mapConnectionTask map[*grpc.ClientConn]*pb.GrepTaskClient) int {
	// Creo una slice con capacità 10 e dimensione iniziale 0
	channels := make([]chan *grpc.ClientConn, 0, Conf.MaxWorkers)

	// Provo con tutte le porte tra [Conf.BaseClientPort, Conf.BaseClientPort + Conf.MaxWorkers]
	for port := Conf.BaseClientPort; port < Conf.BaseClientPort+Conf.MaxWorkers; port++ {
		ch := make(chan *grpc.ClientConn)
		target := fmt.Sprintf("%s:%d", Conf.Address, port)

		// Chiamo una goroutine anonima per connettermi in modo asincrono con tutti i workers disponibili
		go func(target string, channel chan *grpc.ClientConn) {
			// Utilizzo un timeout di un secondo nella goroutine per fare in modo che il master si connetta con l' eventuale worker all'indirizzo target
			timeout, cancelFunc := context.WithTimeout(context.Background(), time.Second)
			defer cancelFunc()
			clientConn, err := grpc.DialContext(timeout, target, grpc.WithInsecure(), grpc.WithBlock()) // CHIAMATA ASINCRONA (senza TLS)
			if err != nil {
				// fmt.Println("Errore nella master goroutine per connettersi a", target, err)
				channel <- nil
				return
			}

			channel <- clientConn
			// fmt.Println("Master gonnection goroutine: SUCCESS for ", target)
		}(target, ch)

		channels = append(channels, ch) // Funziona solo se la slice ha dimensione iniziale 0 e capacity = Conf.MaxWorkers (non funziona con make(..., int) senza 3° parametro)
	}

	for _, ch := range channels {
		conn := <-ch
		if conn != nil {
			//fmt.Printf("Instaurata connessione con worker %d\n", i)
			clientTask := pb.NewGrepTaskClient(conn)
			mapConnectionTask[conn] = &clientTask
		}
	}
	return len(mapConnectionTask)
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
	for connection := range clientMap {
		if connection != nil {
			_ = (*connection).Close()
		}
	}
}

func main() {
	// Prendo i valori dell' indirizzo e della porta da un file di configurazione
	addr := Conf.Address
	port := Conf.MasterPort

	// Prima di andare a servire, mi connetto ai workers disponibili...
	nWorkers := connectToWorkers(mapConnectionTask)
	defer closeConnection(mapConnectionTask)
	fmt.Println("Numero di workers individuati: ", nWorkers)

	// Faccio da server
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", addr, port))
	util.PanicIfMessage(err, "Master: error in Listen()")

	serv := grpc.NewServer()
	pb.RegisterGoGrepServer(serv, &serverMaster{}) // registra i servizi disponibili nel server (vedi file master.proto)

	fmt.Printf("Master server listening at %v\n", lis.Addr())

	// Servo eventuali client.
	err = serv.Serve(lis)
	util.PanicIfMessage(err, fmt.Sprintf("Master: error in Serve()"))
}

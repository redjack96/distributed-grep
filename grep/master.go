package main

import (
	"context"
	"fmt"
	pb "github.com/redjack/distributed_grep/proto"
	"github.com/redjack/distributed_grep/util"
	"google.golang.org/grpc"
	"math"
	"net"
	"os"
	"strings"
	"time"
)

// serverMaster implement proto.MasterGrepServer. Permette la comunicazione RPC con un client. Usata nella main
type serverMaster struct {
	pb.GoGrepServer
	conf        util.Configuration
	workers     int
	channels    []chan *pb.GrepOutput
	tasks       []*pb.GrepTaskClient
	connections []*grpc.ClientConn
}

// fillDefault permette di inserire i valori di default in una struct serverMaster
func (s *serverMaster) fillDefault() {
	s.conf = *util.GetConfig()
	s.channels = make([]chan *pb.GrepOutput, 0, s.conf.MaxWorkers)
	s.tasks = make([]*pb.GrepTaskClient, 0, s.conf.MaxWorkers)
	s.connections = make([]*grpc.ClientConn, 0, s.conf.MaxWorkers)
}

// DistributedGrep - funzione che risponde a un client
func (s *serverMaster) DistributedGrep(ctx context.Context, in *pb.GrepRequest) (*pb.GrepResult, error) {

	fmt.Printf("Avviata grep sul file %s per cercare '%s'\n", in.FilePath, in.Regex) // %v permette di stampare solo i valori di una struct

	// Per le regole di assegnazione dei TASK vedere il README.md

	fileContent, err := os.ReadFile(in.FilePath)
	if err != nil {
		return &pb.GrepResult{Error: err.Error()}, err
	}
	lines := strings.Split(string(fileContent), "\n")
	numLines := len(lines)

	// Scelgo come assegnare il file in base al numero di righe e ai worker disponibili
	taskForMap := int(math.Ceil(math.Min(float64(numLines)/float64(s.conf.RowsPerTask), float64(s.workers))))
	linesPerChunk := int(math.Ceil(float64(numLines) / float64(taskForMap)))

	fmt.Printf("- Linee nel file: \t%d. Linee per chunk Map: \t%d. Numero worker a cui assegnare Map tasks: \t%d\n", len(lines), linesPerChunk, taskForMap)

	/***************** MAP TASK *****************/
	for i := 0; i < taskForMap; i++ {
		var fileChunk string
		if (i+1)*linesPerChunk < len(lines) {
			fileChunk = strings.Join(lines[i*linesPerChunk:(i+1)*linesPerChunk-1], "\n")
		} else {
			fileChunk = strings.Join(lines[i*linesPerChunk:], "\n")
		}
		go s.GoMap(ctx, i, &pb.GrepInput{
			FilePortion: fileChunk,
			Regex:       in.Regex,
			WorkerId:    int32(i),
		})
	}

	// Barriera di sincronizzazione per MAP
	var mapOutput = make([]string, 0, taskForMap)
	for i := 0; i < taskForMap; i++ {
		out := <-s.channels[i]
		if out != nil {
			mapOutput = append(mapOutput, out.Rows)
		}
	}
	mapResult := strings.Join(mapOutput, "")

	linesMap := strings.Split(mapResult, "\n")
	numRigheMap := len(linesMap)
	taskForReduce := int(math.Min(math.Ceil(float64(numRigheMap)/float64(s.conf.RowsPerTask)), float64(s.workers)))
	linesPerChunkReduce := int(math.Ceil(float64(numRigheMap) / float64(taskForReduce)))

	fmt.Printf("- Linee mappate: \t%d. Linee per chunk Reduce: \t%d. Numero worker a cui assegnare Reduce tasks: \t%d\n", numRigheMap, linesPerChunkReduce, taskForReduce)

	/***************** REDUCE TASK *****************/
	for i := 0; i < taskForReduce; i++ {
		var fileChunk string
		if (i+1)*linesPerChunk < len(linesMap) {
			fileChunk = strings.Join(linesMap[i*linesPerChunkReduce:(i+1)*linesPerChunkReduce-1], "\n")
		} else {
			fileChunk = strings.Join(linesMap[i*linesPerChunkReduce:], "\n")
		}
		go s.GoReduce(ctx, i, &pb.GrepInput{
			FilePortion: fileChunk,
			Regex:       in.Regex,
			WorkerId:    int32(i),
		})
		i++
	}

	// Barriera di sincronizzazione del reduce
	var reduceOutput = make([]string, 0, s.conf.MaxWorkers)
	for i := 0; i < taskForReduce; i++ {
		out := <-s.channels[i]
		if out != nil {
			reduceOutput = append(reduceOutput, out.Rows)
		}
	}
	finalResult := strings.Join(reduceOutput, "\n")
	fmt.Printf("Righe nel risultato: %d\n", len(reduceOutput))
	return &pb.GrepResult{
		Rows: finalResult,
	}, nil
}

// GoMap è chiamata dal servizio rpc DistributedGrep ed è eseguita da un goroutine.
// Richiede a sua volta l' esecuzione della rpc GoMap fornita dal worker
func (s *serverMaster) GoMap(ctx context.Context, i int, in *pb.GrepInput) {
	partialOutput, err := (*s.tasks[i]).Map(ctx, in)
	if err != nil {
		s.channels[i] <- &pb.GrepOutput{}
		return
	}
	s.channels[i] <- partialOutput
}

// GoReduce è chiamata dal servizio rpc DistributedGrep ed è eseguita da un goroutine.
// Richiede a sua volta l' esecuzione della rpc GoMap fornita dal worker
func (s *serverMaster) GoReduce(ctx context.Context, i int, in *pb.GrepInput) {
	partialOutput, err := (*s.tasks[i]).Reduce(ctx, in)
	if err != nil {
		s.channels[i] <- &pb.GrepOutput{}
		return
	}
	s.channels[i] <- partialOutput
}

// connectToWorkers cerca e si connette ai worker in listening. Per farlo usa conf.MaxWorkers goroutine che provano
// a connettersi entro un timeout.
func (s *serverMaster) connectToWorkers() {
	// Creo una slice con capacità 10 e dimensione iniziale 0
	channels := make([]chan *grpc.ClientConn, 0, s.conf.MaxWorkers)

	// Provo con tutte le porte tra [conf.BaseWorkerPort, conf.BaseWorkerPort + conf.MaxWorkers]
	for port := s.conf.BaseWorkerPort; port < s.conf.BaseWorkerPort+s.conf.MaxWorkers; port++ {
		ch := make(chan *grpc.ClientConn)
		target := fmt.Sprintf("%s:%d", s.conf.Address, port)

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

		channels = append(channels, ch) // Funziona solo se la slice ha dimensione iniziale 0 e capacity = conf.MaxWorkers (non funziona con make(..., int) senza 3° parametro)
	}
	availableWorkers := 0
	for _, ch := range channels {
		conn := <-ch
		if conn != nil {
			//fmt.Printf("Instaurata connessione con worker %d\n", i)
			clientTask := pb.NewGrepTaskClient(conn)
			s.connections = append(s.connections, conn)
			s.tasks = append(s.tasks, &clientTask)
			s.channels = append(s.channels, make(chan *pb.GrepOutput))
			availableWorkers++
		}
	}
	s.workers = availableWorkers
}

func (s *serverMaster) closeConnection() {
	for _, connection := range s.connections {
		if connection != nil {
			_ = (*connection).Close()
		}
	}
}

func main() {
	// Prendo i valori dell' indirizzo e della porta da un file di configurazione
	s := serverMaster{}
	s.fillDefault()

	addr := s.conf.Address
	port := s.conf.MasterPort

	// Prima di andare a servire, mi connetto ai workers disponibili...
	s.connectToWorkers()
	defer s.closeConnection()
	util.PanicIf(s.workers == 0, "Impossibile trovare worker disponibili. Assicurarsi di avviarne almeno uno.")

	fmt.Println("Numero di workers individuati: ", s.workers)

	// Faccio da server
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", addr, port))
	util.PanicIfMessage(err, "Master: error in Listen()")

	serv := grpc.NewServer()
	pb.RegisterGoGrepServer(serv, &s) // registra i servizi disponibili nel server (vedi file master.proto)

	fmt.Printf("Master server listening at %v\n", lis.Addr())

	// Servo eventuali client.
	err = serv.Serve(lis)
	util.PanicIfMessage(err, fmt.Sprintf("Master: error in Serve()"))
}

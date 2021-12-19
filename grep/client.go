package main

import (
	"context"
	"fmt"
	pb "github.com/redjack/distributed_grep/proto"
	"github.com/redjack/distributed_grep/util"
	"google.golang.org/grpc"
	"os"
	"time"
)

// Esempio di utilizzo del grep distribuito
// Esegue una chiamata a procedura remota (sulla macchina locale)
// Per lanciare via linea di comando, dalla root del progetto esegui "go run grep/main.go"
func main() {
	var argv = os.Args // il nome dell'eseguibile è l'elemento 0
	switch len(argv) {
	case 3:
		startGrep(argv[1], argv[2]) // Come in C, il primo argomento sta a indice 1
	default:
		fmt.Println("Utilizzo:\ndistributed_grep file-path regex") // ci sarebbe anche println(), ma potrebbe essere rimossa nelle versioni successive
	}
}

func startGrep(file, regex string) {

	conf := util.GetConfig()

	clientConn, err := grpc.Dial(fmt.Sprintf("%s:%d", conf.Address, conf.MasterPort), grpc.WithInsecure(), grpc.WithBlock())
	util.PanicOn(err)
	client := pb.NewGoGrepClient(clientConn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*conf.Timeout)
	defer cancel()
	grepResult, err := client.DistributedGrep(ctx, &pb.GrepRequest{
		FilePath: file,
		Regex:    regex,
	})
	util.PanicOn(err)

	if grepResult.Error == "" {
		fmt.Println(grepResult.Rows)
	} else {
		fmt.Println(grepResult.Error)
	}
}

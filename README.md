# Grep Distribuita - Rossi Giacomo Lorenzo - 0292400

Questo progetto implementa una grep distribuita con un architettura master-worker e con il pattern map-reduce.
La comunicazione tra master e worker e tra il programma principale client e il master avviene tramite gRPC.

## Istruzioni per l' esecuzione

- Il progetto è stato testato Windows 10, ma dovrebbe funzionare anche su Linux o Mac
- Se i file *.pb.go generati da protoc non sono presenti, generarli con il seguente comando


    protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto\*.proto

- Dalla root del progetto, lanciare da 1 a w=10 worker in terminali distinti. Il valore w è configurabile nel file config/config.json.


    go run grep/worker.go

- Dalla root del progetto, lanciare un worker in un altro terminale.


    go run grep/master.go    

- Dalla root del progetto, lanciare un client con parametro1 pari al percorso del file e parametro2 la stringa da cercare:

    
    go run grep/client.go

## Regole di assegnazione dei task Map e Reduce ai workers

------
Per assegnare i task ai Worker, il Master cerca di dare lo stesso carico di lavoro a tutti i Worker, evitando però di 
assegnarne troppo poco perché altrimenti l' overhead alla chiamata per parallelizzare sarebbe più lungo del tempo necessario
a svolgere il lavoro in un singolo worker. Per avere vantaggi significativi rispetto alla grep centralizzata, la dimensione
del file di cui eseguire la grep dovrebbe essere tale per cui l' overhead introdotto dalla parallelizzazione e dalle chiamate
RPC sia trascurabile. Perciò la soglia di righe k dopo la quale creare nuovi worker può essere modificata nel file di configurazione.
Siano:
- `L` Il numero di righe contenute nel file di cui eseguire la GREP
- `k=200`(default): numero di righe dopo il quale è necessario creare un nuovo task per i worker.
- `w` i workers disponibili a cui il master può assegnare i task di Map o Reduce.
- `r` il numero di righe che contiene ogni porzione di file da assegnare a tutti i workers tranne l' ultimo.
- `r'`il numero di righe che contiene la porzione di file da assegnare all' ultimo worker


Se il file contiene meno di `k` righe, verrà assegnato a un solo worker un task di Map con `r=k` righe e un task di Reduce.
Se il file contiene più di `k` righe, idealmente il file andrebbe diviso in `n*=math.Ceiling(L/k)` porzioni, ognuna con `r=math.Ceil(L/n)` righe, 
tranne l' ultima porzione che avrà `r'=L-(n-1)*r` righe.

Però non è detto che ci siano sufficienti workers disponibili, quindi il file viene diviso in `n=math.Min(n*, w)` porzioni e si creano `n` mapTask, per altrettanti worker.

### Esempio1 - L = 864, k = 200, w = 5,4
Il numero di task creati sarà:

    n = min(Ceil(864/200), 5) = min(Ceil(4.3), 5) = 5

I primi 4 worker riceveranno in input il seguente numero di righe:

    r = Ceil(864/5) = Ceil(172.8)=173 

Il quinto task riceve 172 righe da cercare

    r' = L - (n-1)*r = 864 - 4*173 = 172

Se ci fossero solo 4 task, ognuno riceverebbe:
		
    r = Ceil(864/4) = 216 righe ciascuno.

Nel codice, l' assegnazione delle righe ai workers `i=0,...,n-1` viene fatta con questo `if`:

    if (i+1)*linesPerChunk < len(lines) {
        chunk = lines[i*linesPerChunk:(i+1)*linesPerChunk-1]
    } else {
        chunk = lines[i*linesPerChunk:]
    }
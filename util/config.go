package util

import (
	"fmt"
	"github.com/tkanos/gonfig"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Configuration struct {
	MasterPort     int    // La porta a cui il master si mette in listening
	BaseWorkerPort int    // La porta a cui il primo worker si mette in listening
	Address        string // L' indirizzo IP a cui il master e il worker si mettono in listening (localhost)
	MaxWorkers     int    // Il numero massimo di workers
	RowsPerTask    int    // Il numero di righe minimo per assegnare un task a un worker. Es: se 200 e il file ha [801-1000] righe si assegnano 5 task Map ai worker
	Timeout        time.Duration
}

// GetConfig Restituisce un puntatore alla struct con i parametri di configurazione nel file config/config.json
func GetConfig() *Configuration {
	configuration := Configuration{}
	err := gonfig.GetConf(getConfigFilePath(), &configuration)
	if err != nil {
		fmt.Println(err)
		os.Exit(500)
	}
	return &configuration
}

// getConfigFilePath permette di ottenere il path assoluto del file di configurazione indipendentemente da
// dove si salva questo progetto e indipendentemente dal SO utilizzato.
func getConfigFilePath() string {
	filename := []string{"../config/", "config", ".json"}
	_, dirname, _, _ := runtime.Caller(0)
	filePath := filepath.ToSlash(path.Join(filepath.Dir(dirname), strings.Join(filename, "")))
	if strings.Contains("windows", runtime.GOOS) {
		strings.Replace(filePath, "/", "\\", -1)
	}
	return filePath
}

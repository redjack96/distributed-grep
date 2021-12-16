package util

import (
	"fmt"
	"github.com/tkanos/gonfig"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type Configuration struct {
	MasterPort     int
	BaseClientPort int
	Address        string
	MaxWorkers     int
}

func GetConfig() (configuration Configuration) {
	configuration = Configuration{}
	err := gonfig.GetConf(GetConfigFilePath(), &configuration)
	if err != nil {
		fmt.Println(err)
		os.Exit(500)
	}
	return
}

func GetConfigFilePath() string {

	filename := []string{"..\\config\\", "config", ".json"}
	_, dirname, _, _ := runtime.Caller(0)
	filePath := filepath.ToSlash(path.Join(filepath.Dir(dirname), strings.Join(filename, "")))
	if strings.Contains("windows", runtime.GOOS) {
		strings.Replace(filePath, "/", "\\", -1)
	}
	fmt.Println("dir: ", filepath.Dir(dirname))
	return filePath
}

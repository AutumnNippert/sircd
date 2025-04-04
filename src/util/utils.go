package util

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
	"encoding/json"
)

type Config struct {
	Host    string `json:"host"`
	Port    string `json:"port"`
	LogFile string `json:"logFile"`
}

var (
	HOST = "localhost"
	PORT = "6667"
	VERSION = "0.1"
	STARTUPTIME = "UNSET"
	// Log file
	LOGFILE = "irc_server.log"
)

func InitConfig() {
	// Load configuration
	config, err := LoadConfig("config.json")
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	HOST = config.Host
	PORT = config.Port
	STARTUPTIME = time.Now().Format("2006-01-02 15:04:05")
	LOGFILE = config.LogFile
}

func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	config := &Config{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	return config, nil
}


func InitLogging(){
	// Open log file
	logFile, err := os.OpenFile(LOGFILE, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// Set output to log file
	log.SetOutput(multiWriter)

	// Set log flags
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Logging started")
}

func GetServerName() string {
	return "sircd"
}
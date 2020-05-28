package tools

import (
	"fmt"
	"io"
	"log"
	"os"
)

// InfoLogger logs main server actions
var InfoLogger *log.Logger

// ErrorLogger logs server errors
var ErrorLogger *log.Logger

const (
	//LogDirectory is a directory path where log files are saved
	logDirectory       = "./logs" // Path : Root/logs
	defaultLogFilePath = "/logfile.txt"
)

// SetUpLogger 는 Exported Logger 오브젝트 생성 및 설정
func SetUpLogger(fileName string) {

	if _, err := os.Stat(logDirectory); os.IsNotExist(err) {
		// rwxrwxrwx (777)
		os.Mkdir(logDirectory, os.ModePerm)
	}

	var logFileName string
	if fileName == "" {
		logFileName = logDirectory + defaultLogFilePath
	} else {
		logFileName = fmt.Sprintf("%s/%s", logDirectory, fileName)
	}

	// Log File 설정
	fpLog, err := os.OpenFile(logFileName,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Log file %s Open error\n", logFileName)
		panic(err)
	}

	// Custom Logger 생성
	InfoLogger = log.New(fpLog, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(fpLog, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	// 파일과 Console에 같이 출력하기 위해 MultiWriter 생성
	multiWriter := io.MultiWriter(fpLog, os.Stdout)
	InfoLogger.SetOutput(multiWriter)
	ErrorLogger.SetOutput(multiWriter)
}

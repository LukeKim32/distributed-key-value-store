package storage

import (
	"bufio"
	"fmt"
	"hash_interface/internal/hash"
	msg "hash_interface/internal/storage/message"
	"hash_interface/tools"
	"log"
	"os"
	"strconv"
	"strings"
)

// dataLoggers gets a logger by passed-key of Each Node address
var dataLoggers map[string] /* key = each Node's address*/ *log.Logger

type logFormat struct {
	KeyValuePair
	Command string
}

type KeyValuePair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

const (
	//LogDirectory is a directory path where log files are saved
	logDirectory = "./internal/cluster/dump"

	// dataLogFormat : 순서대로 (해쉬값, 명령, Key, Value)
	dataLogFormat = "%d %s %s %s"
)

const (
	/* constants for "index" of Data Log each lines */
	hashIndexWord = iota
	commandWord
	keyWord
	valueWord
)

// KeyValueMap : Key -> Value map
type KeyValueMap map[string]string

// HashToDataMap : Hash Index -> (Key -> Value) map
type HashToDataMap map[uint16]KeyValueMap

func init() {

	if _, err := os.Stat(logDirectory); os.IsNotExist(err) {
		// rwxrwxrwx (777)
		os.Mkdir(logDirectory, os.ModePerm)
	}

	if dataLoggers == nil {
		dataLoggers = make(map[string]*log.Logger)
	}

}

// SetUpModificationLogger 는 Data Modification이 일어날 때 파일에 기록을 하기 위한 로거 세터
/*	For Data persistency support
 */
func SetUpModificationLogger(nodeAddressList []string) {

	for _, eachNodeAddress := range nodeAddressList {

		if err := createDataLogFile(eachNodeAddress); err != nil {
			tools.ErrorLogger.Println(msg.CreateLogFileError)
			log.Fatal(err)
		}
	}

}

// createDataLogFile : 각 노드의 주소 = 각 파일명
func createDataLogFile(address string) error {
	filePath := fmt.Sprintf("%s/%s", logDirectory, address)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fpLog, err := os.OpenFile(filePath,
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			panic(err)
		}

		dataLoggers[address] = log.New(fpLog, "", 0)
	}

	return nil
}

func (redisClient RedisClient) RecordModificationLog(command string, key string, value string) error {

	//tools.InfoLogger.Printf(msg.RecordDataLogStart, redisClient.Address)

	targetDataLogger, isSet := dataLoggers[redisClient.Address]
	if isSet == false {
		return fmt.Errorf(msg.DataLoggerSetupError)
	}

	hashSlotIndex := hash.GetHashSlotIndex(key)
	targetDataLogger.Printf(
		dataLogFormat,
		hashSlotIndex,
		command,
		key,
		value,
	)

	return nil
}

// getLatestDataFromLog : 인스턴스의 데이터 로그파일을 읽어 @dataContainer에 (key, value)로 저장한다.
// 동일한 Key 값에 대해서는 최신의 데이터가 저장된다.
//
func (redisClient RedisClient) getLatestDataFromLog(dataContainer HashToDataMap) error {

	//tools.InfoLogger.Printf(msg.ReadDataLogStart, redisClient.Address)

	filePath := fmt.Sprintf("%s/%s", logDirectory, redisClient.Address)
	file, err := os.Open(filePath)
	if err != nil {
		err := fmt.Errorf(msg.DataLogOpenError, filePath, err.Error())
		tools.ErrorLogger.Printf(err.Error())
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// 로그 파일의 끝까지 한 줄 씩 읽는다.
	for scanner.Scan() {

		// 공백 기준으로 Split
		words := strings.Fields(scanner.Text())
		hashIndexIn64, err := strconv.ParseUint(words[hashIndexWord], 10, 16)
		if err != nil {
			return fmt.Errorf(msg.ParseHashIndexStringError)
		}

		hashIndex := uint16(hashIndexIn64)

		// tools.InfoLogger.Printf(
		// 	msg.ReadDataLogEachLine,
		// 	hashIndex,
		// 	words[commandWord],
		// 	words[keyWord],
		// 	words[valueWord],
		// )

		if dataContainer[hashIndex] == nil {
			dataContainer[hashIndex] = make(map[string]string)
		}

		keyValueMap := dataContainer[hashIndex]

		// 데이터 로그 => @dataContainer에 기록
		// 가장 최신의 데이터만 기록에 남음 (이전 데이터 덮어씌움)
		switch words[commandWord] {
		case "SET":
			keyValueMap[words[keyWord]] = words[valueWord]
			break
		case "DEL":
			delete(keyValueMap, words[keyWord])
			break
		default:
			return fmt.Errorf(msg.UnsupportedCommand, words[commandWord])
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf(msg.FileScannerError, err.Error())
	}

	//tools.InfoLogger.Printf("노드(%s)의 데이터 로그 파일 읽기 완료", redisClient.Address)

	return nil
}

// readDataLogs reads Node's data log file and records the information in @hashIndexToKeyValuePairMap
func (redisClient RedisClient) readDataLogs(hashIndexToLogFormatMap map[uint16][]logFormat) error {
	filePath := fmt.Sprintf("%s/%s", logDirectory, redisClient.Address)
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf(msg.DataLogOpenError, filePath, err.Error())
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		words := strings.Fields(scanner.Text())
		hashIndexIn64, err := strconv.ParseUint(words[hashIndexWord], 10, 16)
		if err != nil {
			return fmt.Errorf(msg.ParseHashIndexStringError)
		}

		hashIndex := uint16(hashIndexIn64)

		var logFormat logFormat
		logFormat.Key = words[keyWord]
		logFormat.Value = words[valueWord]
		logFormat.Command = words[commandWord]

		// 공백을 기준으로 split을 하므로, Value 값이 쪼개진 경우 처리
		if len(words) > 4 {
			for i := 4; i < len(words); i++ {
				logFormat.Value += fmt.Sprintf(" %s", words[i])
			}
		}

		// tools.InfoLogger.Printf(
		// 	msg.ReadDataLogEachLine,
		// 	hashIndex,
		// 	words[commandWord],
		// 	words[keyWord],
		// 	words[valueWord],
		// )

		hashIndexToLogFormatMap[hashIndex] = append(
			hashIndexToLogFormatMap[hashIndex],
			logFormat,
		)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf(msg.FileScannerError, err.Error())
	}

	return nil
}

func (redisClient RedisClient) createDataLogFile() error {

	if err := createDataLogFile(redisClient.Address); err != nil {
		return err
	}

	return nil
}

func (redisClient RedisClient) removeDataLogFile() error {
	filePath := fmt.Sprintf("%s/%s", logDirectory, redisClient.Address)

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf(msg.RemoveLogFileError, filePath, err.Error())
	}

	delete(dataLoggers, redisClient.Address)

	return nil
}

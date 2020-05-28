package storage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"hash_interface/configs"
	msg "hash_interface/internal/storage/message"
	"hash_interface/tools"
)

// MonitorClient : Monitor Server들에게 요청을 보낼 Client
type MonitorClient struct {
	ServerAddressList []string
}

var monitorClient MonitorClient

// Question : 모니터 서버에게 요청할 수 있는 내용 옵션 종류
type Question uint8

const (
	IsAlive Question = iota
	NewConnect
	EndConnect
	CurrentList
)

// MonitorServerResponse : Monitor Server의 응답 container
type MonitorServerResponse struct {
	RedisNodeAddress string
	IsAlive          bool
	ErrorMsg         string
	Data             interface{}
}

var errorChannel chan error

func init() {
	if len(monitorClient.ServerAddressList) == 0 {
		monitorClient.ServerAddressList = []string{
			configs.MonitorNodeOneAddress,
			configs.MonitorNodeTwoAddress,
		}
	}
}

// askConnect : 모니터 서버들에게 @redisNode에 대한 연결 setup 요청
//
func (monitorClient MonitorClient) ask(
	redisNode RedisClient,
	question Question,
) (int, error) {

	numberOfmonitorNode := len(monitorClient.ServerAddressList)

	// Buffered Channel : 송신측은 메세지 전송 후 고루틴 계속 진행
	outputChannel := make(
		chan MonitorServerResponse,
		numberOfmonitorNode,
	)

	//tools.InfoLogger.Printf(msg.NewConnectRequest, redisNode.Address)

	votes := 0

	for _, eachMonitorServer := range monitorClient.ServerAddressList {

		switch question {
		case IsAlive:
			// GET
			// URL : http://~/monitor/{redisNodeIp}
			go monitorClient.requestTest(
				eachMonitorServer,
				redisNode.Address,
				outputChannel,
			)
			break

		case NewConnect:
			// POST
			// URL : http://~/monitor/{redisNodeIp}/{role}
			go monitorClient.requestNewConnect(
				eachMonitorServer,
				redisNode.Address,
				outputChannel,
			)
			break

		case EndConnect:
			// DELETE
			// URL : http://~/monitor/{redisNodeIp}
			go monitorClient.requestUnregister(
				eachMonitorServer,
				redisNode.Address,
				outputChannel,
			)
			break

		case CurrentList:
			// TODO
			break
		default:
			// Error
			return -1, fmt.Errorf(msg.UnsupportedMonitorRequest)
		}
	}

	for i := 0; i < numberOfmonitorNode; i++ {

		//tools.InfoLogger.Println(msg.WaitForResponseFromMonitors)

		// 모니터 서버 응답 대기 & 처리
		// To-Do : 모니터 서버 응답이 오지 않는 경우 Timeout 설정
		monitorServerResponse := <-outputChannel
		// tools.InfoLogger.Printf(
		// 	msg.ChannelResponseFromMonitor,
		// 	monitorServerResponse.ErrorMsg,
		// )

		if monitorServerResponse.ErrorMsg != "" {
			tools.ErrorLogger.Printf(
				"모니터 서버 에러 응답 %d : %s",
				i,
				monitorServerResponse.ErrorMsg,
			)
			return -1, fmt.Errorf(monitorServerResponse.ErrorMsg)
		}

		if question == IsAlive {
			if monitorServerResponse.IsAlive {
				// tools.InfoLogger.Printf(
				// 	msg.RedisCheckedAlive,
				// 	redisNode.Address,
				// )

				votes++
			} else {
				tools.InfoLogger.Printf(
					msg.RedisCheckedDead,
					redisNode.Address,
				)
			}
		}
	}

	return votes, nil
}

// requestTest : @redisNodeIp 레디스 노드의 생존여부 @monitorServerIp 해당하는 모니터 서버에 확인 요청
//
func (monitorClient MonitorClient) requestTest(
	monitorServerIp, redisNodeIp string,
	outputChannel chan<- MonitorServerResponse,
) {

	requestURI := fmt.Sprintf(
		"http://%s/monitor/%s",
		monitorServerIp,
		redisNodeIp,
	)

	//tools.InfoLogger.Println(msg.RequestTargetMonitor, requestURI)

	// 서버 요청 타임아웃 설정
	go serverTimoutChecker(monitorServerIp, outputChannel)

	// 모니터 서버에 요청
	response, err := http.Get(requestURI)
	if err != nil {
		tools.ErrorLogger.Printf(
			msg.ResponseMonitorError,
			monitorServerIp,
			err,
		)

		outputChannel <- MonitorServerResponse{
			ErrorMsg: fmt.Sprintf(
				msg.ResponseMonitorError,
				monitorServerIp,
				err,
			),
		}
		return
	}
	defer response.Body.Close()

	// 모니터 서버 응답 파싱
	var monitorServerResponse MonitorServerResponse
	decoder := json.NewDecoder(response.Body)

	if err := decoder.Decode(&monitorServerResponse); err != nil {

		tools.ErrorLogger.Println(
			msg.ResponseMonitorError,
			monitorServerIp,
			monitorServerResponse.ErrorMsg,
		)

		outputChannel <- MonitorServerResponse{
			ErrorMsg: err.Error(),
		}
		return
	}

	// 모니터 서버 테스트 결과 확인
	// tools.InfoLogger.Println(
	// 	msg.ResponseFromTargetMonitor,
	// 	monitorServerResponse.IsAlive,
	// )

	outputChannel <- MonitorServerResponse{
		IsAlive:  monitorServerResponse.IsAlive,
		ErrorMsg: "",
	}
}

func (monitorClient MonitorClient) requestNewConnect(
	monitorServerIp, redisNodeIp string,
	outputChannel chan<- MonitorServerResponse,
) {

	requestURI := fmt.Sprintf(
		"http://%s/monitor/connect/%s",
		monitorServerIp,
		redisNodeIp,
	)

	//tools.InfoLogger.Println(msg.RequestTargetMonitor, requestURI)

	client := &http.Client{}

	registerRequest, err := http.NewRequest(
		http.MethodPost,
		requestURI,
		nil,
	)

	if err != nil {
		tools.ErrorLogger.Printf(
			msg.CreateRequestError,
			monitorServerIp,
			err,
		)

		outputChannel <- MonitorServerResponse{
			ErrorMsg: fmt.Sprintf(
				msg.CreateRequestError,
				monitorServerIp,
				err,
			),
		}
		return
	}

	// 서버 요청 타임아웃 설정
	go serverTimoutChecker(monitorServerIp, outputChannel)

	response, err := client.Do(registerRequest)
	if err != nil {
		tools.ErrorLogger.Printf(
			msg.ResponseMonitorError,
			monitorServerIp,
			err,
		)

		outputChannel <- MonitorServerResponse{
			ErrorMsg: fmt.Sprintf(
				msg.ResponseMonitorError,
				monitorServerIp,
				err,
			),
		}
		return
	}
	defer response.Body.Close()

	// 모니터 서버 응답 파싱
	var monitorServerResponse MonitorServerResponse
	decoder := json.NewDecoder(response.Body)

	if err := decoder.Decode(&monitorServerResponse); err != nil {
		tools.ErrorLogger.Println(
			msg.ResponseMonitorError,
			monitorServerIp,
			monitorServerResponse.ErrorMsg,
		)

		outputChannel <- MonitorServerResponse{
			ErrorMsg: err.Error(),
		}
		return
	}

	outputChannel <- MonitorServerResponse{
		ErrorMsg: monitorServerResponse.ErrorMsg,
	}
}

func serverTimoutChecker(
	monitorServerIp string,
	outputChannel chan<- MonitorServerResponse,
) {

	time.Sleep(3 * time.Second)

	outputChannel <- MonitorServerResponse{
		ErrorMsg: fmt.Sprintf(
			msg.MonitorRequestTimeout,
			monitorServerIp,
		),
	}

}

func (monitorClient MonitorClient) requestUnregister(
	monitorServerIp, redisNodeIp string,
	outputChannel chan<- MonitorServerResponse,
) {

	requestURI := fmt.Sprintf(
		"http://%s/monitor/connect/%s",
		monitorServerIp,
		redisNodeIp,
	)

	//tools.InfoLogger.Println(msg.RequestTargetMonitor, requestURI)

	client := &http.Client{}

	req, err := http.NewRequest(
		http.MethodDelete,
		requestURI,
		nil,
	)
	if err != nil {
		tools.ErrorLogger.Printf(
			msg.CreateRequestError,
			err,
		)

		outputChannel <- MonitorServerResponse{
			ErrorMsg: fmt.Sprintf(
				msg.CreateRequestError,
				monitorServerIp,
				err,
			),
		}
		return
	}

	// 서버 요청 타임아웃 설정
	go serverTimoutChecker(monitorServerIp, outputChannel)

	// 모니터 서버에 요청
	response, err := client.Do(req)
	if err != nil {
		tools.ErrorLogger.Printf(
			msg.ResponseMonitorError,
			monitorServerIp,
			err,
		)

		outputChannel <- MonitorServerResponse{
			ErrorMsg: fmt.Sprintf(
				msg.ResponseMonitorError,
				monitorServerIp,
				err,
			),
		}
		return
	}
	defer response.Body.Close()

	// 모니터 서버 응답 파싱
	var monitorServerResponse MonitorServerResponse
	decoder := json.NewDecoder(response.Body)

	if err := decoder.Decode(&monitorServerResponse); err != nil {
		tools.ErrorLogger.Println(
			msg.ResponseMonitorError,
			monitorServerIp,
			monitorServerResponse.ErrorMsg,
		)

		outputChannel <- MonitorServerResponse{
			ErrorMsg: err.Error(),
		}
		return
	}

	outputChannel <- MonitorServerResponse{
		ErrorMsg: monitorServerResponse.ErrorMsg,
	}
}

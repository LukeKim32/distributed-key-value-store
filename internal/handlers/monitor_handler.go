package handlers

import (
	"fmt"
	"hash_interface/configs"
	"hash_interface/internal/models/response"
	"hash_interface/internal/storage"
	"hash_interface/tools"

	"encoding/json"
	"net/http"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
)

// CheckRedisNodeStatus
func CheckRedisNodeStatus(res http.ResponseWriter, req *http.Request) {

	pathVars := mux.Vars(req)
	targetRedisAddress := pathVars["redis_address"]
	checkResult := storage.MonitorServerResponse{
		RedisNodeAddress: targetRedisAddress,
		ErrorMsg:         "",
		IsAlive:          true,
	}

	redisClient, err := storage.GetMasterWithAddress(targetRedisAddress)
	if err != nil {
		checkResult.IsAlive = false
		checkResult.ErrorMsg = err.Error()

		tools.InfoLogger.Printf(
			"CheckRedisNodeStatus() : 레디스 %s 없음",
			targetRedisAddress,
		)

		monitorResponseError(res, err)
		return

	}

	tools.InfoLogger.Printf(
		"CheckRedisNodeStatus() : 레디스 %s 발견",
		targetRedisAddress,
	)

	// 연결인 안되어있는 경우 1.
	// 레디스 컨테이너가 죽었다 살아난 경우, 기존 Connection = nil
	if redisClient.Connection == nil {

		err = fmt.Errorf(
			"레디스(%s) Ping test 실패, connection nil",
			targetRedisAddress,
		)

		tools.ErrorLogger.Printf(err.Error())

		monitorResponseError(res, err)
		return
	}

	result, err := redis.String(redisClient.Connection.Do("PING"))

	// 연결인 안되어있는 경우 2.
	// 레디스 컨테이너 죽어있는 경우
	if err != nil {
		err = fmt.Errorf(
			"레디스(%s) Ping test 실패",
			targetRedisAddress,
		)

		tools.ErrorLogger.Printf(err.Error())

		monitorResponseError(res, err)
		return
	}

	tools.InfoLogger.Println("CheckRedisNodeStatus() Result : ", result)

	responseWithCurrentRedisList(res, checkResult, "CheckRedisNodeStatus")
}

func UnregisterRedis(res http.ResponseWriter, req *http.Request) {

	pathVars := mux.Vars(req)
	targetRedisAddress := pathVars["redis_address"]

	targetRedisClient := storage.RedisClient{
		Address: targetRedisAddress,
		Role:    storage.MasterRole, // 모니터 서버는 모든 레디스를 마스터로 관리
	}

	tools.InfoLogger.Printf("UnregisterRedis() : 레다스(%s) 삭제 시작")

	targetRedisClient.RemoveFromList()

	responseBody := storage.MonitorServerResponse{
		RedisNodeAddress: targetRedisAddress,
		ErrorMsg:         "",
	}

	responseWithCurrentRedisList(res, responseBody, "UnregisterRedis")
}

func RegisterNewRedis(res http.ResponseWriter, req *http.Request) {

	pathVars := mux.Vars(req)
	targetRedisAddress := pathVars["redis_address"]

	responseBody := storage.MonitorServerResponse{
		RedisNodeAddress: targetRedisAddress,
		ErrorMsg:         "",
	}

	var err error

	var newRedisClient storage.RedisClient

	newRedisClient.Connection, err = redis.Dial(
		"tcp",
		targetRedisAddress,
		redis.DialConnectTimeout(storage.ConnTimeoutDuration),
	)

	if err != nil {
		tools.ErrorLogger.Printf(
			"registerNewRedis() : 새로운 레디스 노드 (%s) 추가 실패 - %s",
			targetRedisAddress,
			err.Error(),
		)

		monitorResponseError(res, err)
		return

	}

	// 연결 성공시
	newRedisClient.Role = storage.MasterRole
	newRedisClient.Address = targetRedisAddress
	storage.AppendMaster(newRedisClient)

	responseWithCurrentRedisList(res, responseBody, "RegisterNewRedis")
}

func ShowCurrentRedisList(res http.ResponseWriter, req *http.Request) {

	responseBody := storage.MonitorServerResponse{}

	responseWithCurrentRedisList(res, responseBody, "ShowCurrentRedisList")
}

// responseWithCurrentRedisList : 모니터 서버는 모든 레디스 노드들을 마스터 노드로 관리
//
func responseWithCurrentRedisList(res http.ResponseWriter, checkResult storage.MonitorServerResponse, handleFuncName string) {

	checkResult.Data = response.RedisListTemplate{
		Masters: storage.GetMasterClients(),
	}

	responseBody, err := json.Marshal(checkResult)
	if err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	tools.InfoLogger.Printf(
		"%s() Response : %s",
		handleFuncName,
		string(responseBody),
	)

	responseOK(res, responseBody)
}

func monitorResponseError(res http.ResponseWriter, err error) {
	checkResult := storage.MonitorServerResponse{
		ErrorMsg: err.Error(),
	}

	responseBody, encodErr := json.Marshal(checkResult)
	if encodErr != nil {
		responseError(res, http.StatusInternalServerError, encodErr)
		return
	}

	tools.InfoLogger.Printf(
		"모니터 서버 에러 : 에러내용 : %s",
		err.Error(),
	)

	res.Header().Set(configs.ContentType, configs.JsonContent)
	res.WriteHeader(http.StatusInternalServerError)
	fmt.Fprint(res, string(responseBody))
}

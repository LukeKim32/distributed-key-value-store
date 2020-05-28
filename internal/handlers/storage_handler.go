package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"hash_interface/configs"
	"hash_interface/internal/cluster"
	"hash_interface/internal/hash"
	"hash_interface/internal/models"
	"hash_interface/internal/models/response"
	"hash_interface/internal/storage"
	"hash_interface/tools"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
)

func HandleUpdateKeyValue(res http.ResponseWriter, req *http.Request) {

	stateNode := cluster.StateNode

	freePass := req.Header.Get(cluster.PassToStoreHeader)

	if freePass != "" {
		indexTime := stateNode.GetIndexTime(true)
		indexTimeString := fmt.Sprintf(
			"%d",
			indexTime,
		)

		res.Header().Set(
			cluster.IndexTimeHeader,
			indexTimeString,
		)

		SetKeyValue(res, req)
		return
	}

	// 1. 요청이 리더로부터 왔는지 유저에게 왔는지 확인

	eventDispatcher := cluster.EventDispatcher

	requestedData := models.DataRequestContainer{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&requestedData)
	if err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	keyValuePair := requestedData.Data[0]

	// interruptChannel은 커널이 인터럽트를 발생하여 IO가 끝난 것을 알려주듯
	// State Machine의 로직이 끝나는 것을 알림받는 채널
	//
	interruptChannel := make(chan error)
	metaDataMap := extractMetaData(req)

	// 리더가 없는 경우 리더가 생길 때 까지 Busy Waiting
	// 1. Cold start : 제일 처음 시작했을 때, Follower로 등록되어있을 떄 요청이 온 경우
	//
	// 2. 자신이 Candidate일 때
	if stateNode.HasNoLeader() {
		stateNode.WaitForNewLeader()
	}

	// 여기서 리더가 없어졌을 경우
	// 이 Write Request는 손실될 수 있다.

	if stateNode.IsMyselfLeader() {

		eventDispatcher.DispatchAppendWal(
			keyValuePair,
			metaDataMap,
			&interruptChannel,
		)

	} else {

		eventDispatcher.DispatchPassToLeader(
			keyValuePair.Key,
			keyValuePair.Value,
			&interruptChannel,
		)

	}

	err = <-interruptChannel

	if err != nil {
		responseError(res, http.StatusBadRequest, err)
		return
	}

	indexTime := fmt.Sprintf(
		"%d",
		stateNode.GetIndexTime(true),
	)

	res.Header().Set(
		cluster.IndexTimeHeader,
		indexTime,
	)

	responseTemplate := response.BasicTemplate{}
	curMsg := fmt.Sprintf(
		"SET completed Success : Handled in Server(IP : %s)",
		configs.CurrentIP,
	)
	nextMsg := "Main URL"
	nextLink := configs.HTTP + configs.BaseURL

	responseBody, err := responseTemplate.Marshal(
		curMsg,
		nextMsg,
		nextLink,
	)
	if err != nil {
		tools.ErrorLogger.Println(err.Error())
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	responseOK(res, responseBody)

}

func extractMetaData(req *http.Request) map[string]interface{} {

	uintFields := []string{
		cluster.StartIndexTimeHeader,
		cluster.IndexTimeHeader,
		cluster.TermHeader,
	}

	metaDataMap := make(map[string]interface{})

	for _, eachField := range uintFields {

		valueInString := req.Header.Get(eachField)
		if valueInString != "" {
			value, err := strconv.ParseUint(
				valueInString,
				10,
				64,
			)
			if err == nil {
				metaDataMap[eachField] = value
			}
		}
	}

	stringFields := []string{
		cluster.LeaderHeader,
	}

	for _, eachField := range stringFields {

		value := req.Header.Get(eachField)
		if value != "" {
			metaDataMap[eachField] = value
		}
	}

	// for key, value := range metaDataMap {

	// 	if value == nil {

	// 		tools.InfoLogger.Printf(
	// 			"HTTP 요청에서 메타데이터 뽑기 - key : %s, value : nil",
	// 			key,
	// 		)

	// 	} else {

	// 		tools.InfoLogger.Println(
	// 			"HTTP 요청에서 메타데이터 뽑기 - key : ",
	// 			key,
	// 			" value : ",
	// 			value,
	// 		)
	// 	}
	// }

	return metaDataMap
}

// 클러스터 내부 메세지 통해서만 직접 저장 가능
//
func SetKeyValue(res http.ResponseWriter, req *http.Request) {

	// To check if load balancing(Round-robin) works
	tools.InfoLogger.Printf(
		"SetKeyValue() 로 전달!! Interface server(IP : %s) Processing...\n",
		configs.CurrentIP,
	)

	// 요청 Body 파싱
	var DataRequestContainer models.DataRequestContainer
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&DataRequestContainer)
	if err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	var responseTemplate response.SetResultTemplate
	responseTemplate.Results = make(
		[]response.RedisResult,
		len(DataRequestContainer.Data),
	)

	// 각 Set 요청 Key, Value
	for i, eachKeyValue := range DataRequestContainer.Data {

		key := eachKeyValue.Key
		value := eachKeyValue.Value
		hashSlotIndex := hash.GetHashSlotIndex(key)

		tools.InfoLogger.Printf(
			"SET Key : %s, Value : %s - 해쉬 슬롯 : %d",
			key,
			value,
			hashSlotIndex,
		)

		// Key의 해쉬 슬롯을 담당하는 레디스 획득
		// Get Redis Node which handles this hash slot
		redisClient, err := storage.GetRedisClient(hashSlotIndex)
		if err != nil {
			responseError(res, http.StatusInternalServerError, err)
			return
		}

		// 레디스에 요청 명령 실행
		_, err = redis.String(redisClient.Connection.Do("SET", key, value))
		if err != nil {
			responseError(res, http.StatusInternalServerError, err)
			return
		}

		// 변경사항 데이터 로그 기록
		err = redisClient.RecordModificationLog("SET", key, value)
		if err != nil {
			responseError(res, http.StatusInternalServerError, err)
			return
		}

		// 슬레이브에게 전파
		redisClient.ReplicateToSlave("SET", key, value)

		responseTemplate.Results[i].NodeAdrress = redisClient.Address
		responseTemplate.Results[i].Result = fmt.Sprintf(
			"%s %s %s",
			"SET",
			key,
			value,
		)
	}

	curMsg := fmt.Sprintf(
		"SET completed Success : Handled in Server(IP : %s)",
		configs.CurrentIP,
	)
	nextMsg := "Main URL"
	nextLink := configs.HTTP + configs.BaseURL

	responseBody, err := responseTemplate.Marshal(curMsg, nextMsg, nextLink)
	if err != nil {
		tools.ErrorLogger.Println(err.Error())
		return
	}

	responseOK(res, responseBody)
}

// GetValueFromKey is a handler function for @GET, processing the reqeust
// URI로 전달받은 Key값을 가져온다.
//

// @Summary Get stored Value with passed Key
// @Description ## 요청한 Key 값에 저장된 Value 값 가져오기
// @Accept json
// @Produce json
// @Router /hash/data/{key} [get]
// @Param key path string true "Target Key"
// @Success 200 {object} response.GetResultTemplate
// @Failure 500 {object} response.BasicTemplate "서버 오류"
func GetValueFromKey(res http.ResponseWriter, req *http.Request) {

	// To check if load balancing(Round-robin) works
	// tools.InfoLogger.Printf(
	// 	"Interface server(IP : %s) Processing...\n",
	// 	configs.CurrentIP,
	// )

	reqIdxTimeString := req.Header.Get(cluster.IndexTimeHeader)
	reqIdxTime, err := strconv.ParseUint(
		reqIdxTimeString,
		10,
		64,
	)

	if err != nil {
		err = fmt.Errorf(
			"Request Header에 Index Time을 올바르게 설정해주세요",
		)
		responseError(res, http.StatusBadRequest, err)
		return
	}

	stateNode := cluster.StateNode

	// !!밸런서가 리더에게 알맞은 Index Time을 알아서 물어보고 요청을 날린다고 가정!!!

	// 요청의 Index Time은 잘못된 것이 아니므로,
	// 현재 IndexTime이 뒤쳐져 있을 경우
	// (업데이트를 뒤에서 - heartbeat로 할테니깐)
	// Busy Waiting을 하자
	for reqIdxTime > stateNode.GetIndexTime(true) {
	}

	// Commit Index와 Index Time이 별개이기 때문에
	// 원래는 Commit Index와 Request Index Tim이 같아질때까지 기다려야한다!!

	params := mux.Vars(req)
	key := params["key"]
	hashSlotIndex := hash.GetHashSlotIndex(key)

	// Key의 해쉬 슬롯을 담당하는 레디스 획득
	redisClient, err := storage.GetRedisClient(hashSlotIndex)
	if err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	// 레디스에 요청 명령 실행
	redisResponse, err := redis.String(redisClient.Connection.Do("GET", key))
	if err == redis.ErrNil {
		redisResponse = "nil(없음)"

	} else if err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	curMsg := fmt.Sprintf(
		"GET %s completed Success : Handled in Server(IP : %s)",
		key,
		configs.CurrentIP,
	)
	nextMsg := "Main URL"
	nextLink := configs.HTTP + configs.BaseURL

	responseTemplate := response.GetResultTemplate{}
	responseTemplate.Result = redisResponse
	responseTemplate.NodeAdrress = redisClient.Address

	responseBody, err := responseTemplate.Marshal(
		redisResponse,
		redisClient.Address,
		curMsg, nextMsg, nextLink,
	)
	if err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	responseOK(res, responseBody)
}

// @Summary Add New Master/Slave Redis Clients
// @Description **Slave 추가 시,** 반드시 요청 바디에 **"master_address" 필드에 타겟 노드 주소 설정**
// @Description Master, Slave 운용하고 싶지 않은 경우, 모두 Master로 등록
// @Accept json
// @Produce json
// @Router /clients [post]
// @Param newSetData body models.NewClientRequestContainer true "Specifying Role and Address of New Node"
// @Success 200 {object} response.RedisListTemplate
// @Failure 500 {object} response.BasicTemplate "서버 오류"
func AddNewStorage(res http.ResponseWriter, req *http.Request) {

	// To check if load balancing(Round-robin) works
	//tools.InfoLogger.Printf("Interface server(IP : %s) Processing...\n", configs.CurrentIP)

	// 요청 Body 파싱
	var newClientRequest models.NewClientRequestContainer
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&newClientRequest); err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	// 요청 오류 체크
	if newClientRequest.IsEmpty() {
		err := fmt.Errorf("AddNewStorage() : request body of 'client' is empty")
		responseError(res, http.StatusBadRequest, err)
		return
	}

	switch newClientRequest.Role {
	case storage.MasterRole:
		err := storage.AddNewMaster(newClientRequest.Address)
		if err != nil {
			responseError(res, http.StatusInternalServerError, err)
			return
		}

	case storage.SlaveRole:

		// 타겟 마스터 확인
		if newClientRequest.MasterAddress == "" {
			err := fmt.Errorf("마스터 주소 없음")
			tools.ErrorLogger.Printf(
				"AddNewStorage() : 슬레이브 추가 에러 - %s",
				err.Error(),
			)
			responseError(res, http.StatusBadRequest, err)
			return
		}

		targetMaster, err := storage.GetMasterWithAddress(newClientRequest.MasterAddress)
		if err != nil {
			tools.ErrorLogger.Printf(
				"AddNewStorage() : 슬레이브 추가 에러 - %s",
				err.Error(),
			)
			responseError(res, http.StatusBadRequest, err)
			return
		}

		err = storage.AddNewSlave(newClientRequest.Address, *targetMaster)
		if err != nil {
			tools.ErrorLogger.Printf(
				"AddNewStorage() : 슬레이브 추가 에러 - %s",
				err.Error(),
			)
			responseError(res, http.StatusInternalServerError, err)
			return
		}

	default:
		err := fmt.Errorf("AddNewStorage() : 지원하지 않는 %s role", newClientRequest.Role)
		tools.ErrorLogger.Printf(err.Error())
		responseError(res, http.StatusBadRequest, err)
		return
	}

	responseTemplate := response.RedisListTemplate{
		Masters: storage.GetMasterClients(),
		Slaves:  storage.GetSlaveClients(),
	}

	curMsg := fmt.Sprintf(
		"신규 레디스(%s) (역할 : %s) 등록 성공",
		newClientRequest.Address,
		newClientRequest.Role,
	)
	nextMsg := "Main URL"
	nextLink := configs.HTTP + configs.BaseURL

	// JSON marshaling(Encoding to Bytes)
	responseBody, err := responseTemplate.Marshal(curMsg, nextMsg, nextLink)
	if err != nil {
		tools.ErrorLogger.Println(err.Error())
		return
	}

	responseOK(res, responseBody)
}

// @Summary Get Currently Registered Master/Slave Redis Clients
// @Accept json
// @Produce json
// @Router /clients [get]
// @Success 200 {object} response.RedisListTemplate
// @Failure 500 {object} response.BasicTemplate "서버 오류"
func GetStorageInfo(res http.ResponseWriter, req *http.Request) {

	// To check if load balancing(Round-robin) works
	//tools.InfoLogger.Printf("Interface server(IP : %s) Processing...\n", configs.CurrentIP)

	responseTemplate := response.RedisListTemplate{
		Masters: storage.GetMasterClients(),
		Slaves:  storage.GetSlaveClients(),
	}

	curMsg := fmt.Sprintf(
		"현재 레디스 클라이언트",
	)
	nextMsg := "Main URL"
	nextLink := configs.HTTP + configs.BaseURL

	// JSON marshaling(Encoding to Bytes)
	responseBody, err := responseTemplate.Marshal(curMsg, nextMsg, nextLink)
	if err != nil {
		tools.ErrorLogger.Println(err.Error())
		return
	}

	responseOK(res, responseBody)

}

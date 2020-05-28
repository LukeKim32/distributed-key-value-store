package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"hash_interface/configs"
	"hash_interface/internal/cluster"
	"hash_interface/internal/models"
	"hash_interface/internal/models/response"
	"hash_interface/tools"

	"github.com/gorilla/mux"
)

// @Summary Add New Master/Slave Redis Clients
// @Description **Slave 추가 시,** 반드시 요청 바디에 **"master_address" 필드에 타겟 노드 주소 설정**
// @Description Master, Slave 운용하고 싶지 않은 경우, 모두 Master로 등록
// @Accept json
// @Produce json
// @Router /clients [post]
// @Param newSetData body models.NewClientRequestContainer true "Specifying Role and Address of New Node"
// @Success 200 {object} response.RedisListTemplate
// @Failure 500 {object} response.BasicTemplate "서버 오류"

type Register struct {
	Address string
}

func RegisterNode(res http.ResponseWriter, req *http.Request) {

	requestData := Register{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&requestData)
	if err != nil {
		responseError(res, http.StatusBadRequest, err)
		return
	}

	anotherHost := requestData.Address
	queryStrings := req.URL.Query()
	handshakeOption := queryStrings["handshake"]
	startPointOption := queryStrings["startPoint"]

	// 요청 오류 체크
	if anotherHost == "" {
		err := fmt.Errorf("RegisterNode() : 등록 호스트 주소 에러")
		responseError(res, http.StatusBadRequest, err)
		return
	}

	stateNode := cluster.StateNode

	if len(handshakeOption) < 1 {
		err := fmt.Errorf("Handshake 옵션 미설정")
		responseError(res, http.StatusBadRequest, err)
		return
	}

	if len(startPointOption) < 1 {
		err := fmt.Errorf("startPoint 옵션 미설정")
		responseError(res, http.StatusBadRequest, err)
		return
	}

	isHandshakeOn, err := strconv.ParseBool(handshakeOption[0])
	if err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	isStartPoint, err := strconv.ParseBool(startPointOption[0])
	if err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	if isStartPoint {
		err = stateNode.Register(anotherHost)
		if err != nil {
			responseError(res, http.StatusInternalServerError, err)
			return
		}

	} else {

		if isHandshakeOn {
			err = stateNode.SendRegisterMsg(
				anotherHost,
				stateNode.GetCurServerIP(),
				cluster.NoHandShake,
				cluster.NoStartPoint,
			)
			if err != nil {
				responseError(res, http.StatusInternalServerError, err)
				return
			}
		}

	}

	stateNode.AddNewNode(anotherHost)

	responseTemplate := response.BasicTemplate{}

	curMsg := fmt.Sprintf(
		"클러스터 노드(%s) 등록 성공",
		anotherHost,
	)
	nextMsg := "Main URL"
	nextLink := configs.HTTP + configs.BaseURL

	// JSON marshaling(Encoding to Bytes)
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

// @Summary Get Currently Registered Master/Slave Redis Clients
// @Accept json
// @Produce json
// @Router /clients [get]
// @Success 200 {object} response.RedisListTemplate
// @Failure 500 {object} response.BasicTemplate "서버 오류"
func StartCluster(res http.ResponseWriter, req *http.Request) {

	queryStrings := req.URL.Query()
	startPointOption := queryStrings["startPoint"]

	if len(startPointOption) < 1 {
		err := fmt.Errorf("startPoint 옵션 미설정")
		responseError(res, http.StatusBadRequest, err)
		return
	}

	isStartPoint, err := strconv.ParseBool(startPointOption[0])
	if err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	// 클러스터 노드가 3개 이상, 홀수 개인지 확인
	stateNode := cluster.StateNode

	IsClusterStartable := stateNode.IsStartable()

	if IsClusterStartable != nil {

		responseError(
			res,
			http.StatusBadRequest,
			IsClusterStartable,
		)
		return
	}

	stateNode.Start(isStartPoint)

	responseTemplate := response.BasicTemplate{}

	curMsg := "클러스터 시작"
	nextMsg := "Main URL"
	nextLink := configs.HTTP + configs.BaseURL

	// JSON marshaling(Encoding to Bytes)
	responseBody, err := responseTemplate.Marshal(curMsg, nextMsg, nextLink)
	if err != nil {
		tools.ErrorLogger.Println(err.Error())
		responseError(res, http.StatusInternalServerError, err)
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
func PrintRegisteredNodes(res http.ResponseWriter, req *http.Request) {

	stateNode := cluster.StateNode

	responseTemplate := response.ClusterNodeListTemplate{}
	curMsg := fmt.Sprintf(
		"현재 노드(%s)에 등록된 클러스터 노드",
		stateNode.GetCurServerIP(),
	)
	nextMsg := "Main URL"
	nextLink := configs.HTTP + configs.BaseURL

	// JSON marshaling(Encoding to Bytes)
	responseBody, err := responseTemplate.Marshal(
		stateNode.GetNodeAddressList(),
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

// @Summary Get Currently Registered Master/Slave Redis Clients
// @Accept json
// @Produce json
// @Router /clients [get]
// @Success 200 {object} response.RedisListTemplate
// @Failure 500 {object} response.BasicTemplate "서버 오류"
func PrintLeader(res http.ResponseWriter, req *http.Request) {

	stateNode := cluster.StateNode

	responseTemplate := response.BasicTemplate{}
	nextMsg := "Main URL"
	nextLink := configs.HTTP + configs.BaseURL

	// JSON marshaling(Encoding to Bytes)
	responseBody, err := responseTemplate.Marshal(
		stateNode.LeaderAddress,
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

// @Summary Get Currently Registered Master/Slave Redis Clients
// @Accept json
// @Produce json
// @Router /clients [get]
// @Success 200 {object} response.RedisListTemplate
// @Failure 500 {object} response.BasicTemplate "서버 오류"
func Vote(res http.ResponseWriter, req *http.Request) {

	termInString := req.Header.Get(cluster.TermHeader)
	if termInString == "" {
		err := fmt.Errorf("term 미설정")
		responseError(res, http.StatusBadRequest, err)
		return
	}

	reqTerm, err := strconv.ParseUint(
		termInString,
		10,
		64,
	)

	if err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	eventDispatcher := cluster.EventDispatcher

	interruptChannel := make(chan error)

	eventDispatcher.DispatchVote(
		reqTerm,
		&interruptChannel,
	)

	err = <-interruptChannel
	if err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	responseOK(res, []byte{})
}

// 리더가 follower로부터 전달받는 append
//
func HandleAppendWal(res http.ResponseWriter, req *http.Request) {

	tools.InfoLogger.Println(
		"Follower로부터 Append Entry 전달받음!",
	)

	eventDispatcher := cluster.EventDispatcher

	requestData := models.DataRequestContainer{}
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&requestData); err != nil {
		tools.InfoLogger.Println(
			"전달받은 AppendWal 에러 발생",
		)
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	keyValuePair := requestData.Data[0]
	metaDataMap := extractMetaData(req)
	interruptChannel := make(chan error)

	eventDispatcher.DispatchAppendWal(
		keyValuePair,
		metaDataMap,
		&interruptChannel,
	)

	err := <-interruptChannel
	if err != nil {
		responseError(res, http.StatusBadRequest, err)
		return

	}

	responseOK(res, []byte{})
}

func HandleHeartbeat(res http.ResponseWriter, req *http.Request) {

	/* 모든 핸들러에다가 넣기 - 스테이트 노드의 초기 시작 = Stopped 상태일경우
	StartEventLoop() 하기
	*/

	isInternalCommunication := req.Header.Get(
		cluster.InternalTokenHeader,
	)

	if isInternalCommunication == "" {
		err := fmt.Errorf("허용되지 않은 요청입니다")
		responseError(res, http.StatusUnauthorized, err)
		return
	}

	leaderTermString := req.Header.Get(
		cluster.TermHeader,
	)

	if leaderTermString == "" {
		err := fmt.Errorf("Heartbeat가 잘못되었습니다")
		responseError(res, http.StatusBadRequest, err)
		return
	}

	leader := req.Header.Get(
		cluster.LeaderHeader,
	)

	leaderTerm, err := strconv.ParseUint(
		leaderTermString,
		10,
		64,
	)
	if err != nil {
		responseError(res, http.StatusBadRequest, err)
		return
	}

	indexTimeString := req.Header.Get(
		cluster.IndexTimeHeader,
	)

	indexTime, err := strconv.ParseUint(
		indexTimeString,
		10,
		64,
	)
	if err != nil {
		responseError(res, http.StatusBadRequest, err)
		return
	}

	commitIndexString := req.Header.Get(
		cluster.CommitIndexHeader,
	)

	commitIndex, err := strconv.ParseUint(
		commitIndexString,
		10,
		64,
	)
	if err != nil {
		responseError(res, http.StatusBadRequest, err)
		return
	}

	eventDispatcher := cluster.EventDispatcher

	interruptChannel := make(chan error)

	stateNode := cluster.StateNode

	if !stateNode.IsRunning() {
		stateNode.Start(false)
	}

	eventDispatcher.DispatchHeartbeat(
		leader,
		leaderTerm,
		indexTime,
		commitIndex,
		&interruptChannel,
	)

	<-interruptChannel

	responseOK(res, []byte{})
}

// 리더에게만 오는 요청
// 요청(origin)에게 wal을 전달해준다
//
func StartUpdateFollowerWal(res http.ResponseWriter, req *http.Request) {

	tools.InfoLogger.Printf(
		"팔로워로부터 업데이트 해달라는 요청을 받음!",
	)

	follower := req.Header.Get(cluster.OriginHeader)
	followerIdxTimeString := req.Header.Get(cluster.IndexTimeHeader)

	if follower == "" || followerIdxTimeString == "" {
		err := fmt.Errorf(
			"Follower의 주소 또는 Index Time이 설정되지 않았습니다",
		)

		tools.InfoLogger.Printf(
			"Follower의 주소 또는 Index Time이 설정되지 않았습니다",
		)

		responseError(res, http.StatusBadRequest, err)
		return
	}

	followerIdxTime, err := strconv.ParseUint(
		followerIdxTimeString,
		10,
		64,
	)
	if err != nil {
		tools.InfoLogger.Printf(
			"Follower의 Index Time 파싱 에러",
		)

		responseError(res, http.StatusBadRequest, err)
		return
	}

	eventDispatcher := cluster.EventDispatcher
	interruptChannel := make(chan error)

	eventDispatcher.DispatchStartUpdateFollowerWal(
		follower,
		followerIdxTime,
		&interruptChannel,
	)
	err = <-interruptChannel

	if err != nil {

		tools.InfoLogger.Printf(
			"Follower 업데이트 중 에러! %s",
			err.Error(),
		)
		responseError(res, http.StatusBadRequest, err)
		return
	}

	responseOK(res, []byte{})
}

// 리더에게 요청한 WAL 데이터 저장
//
func UpdateWalFromLeader(res http.ResponseWriter, req *http.Request) {

	targetIdxString := req.Header.Get(cluster.IndexTimeHeader)

	if targetIdxString == "" {
		err := fmt.Errorf(
			"리더의 Index Time 미설정",
		)
		responseError(res, http.StatusBadRequest, err)
		return
	}

	targetIdx, err := strconv.ParseUint(
		targetIdxString,
		10,
		64,
	)
	if err != nil {
		responseError(res, http.StatusBadRequest, err)
		return
	}

	updatedData := models.DataRequestContainer{}
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&updatedData); err != nil {
		responseError(res, http.StatusInternalServerError, err)
		return
	}

	keyValuePair := updatedData.Data[0]

	eventDispatcher := cluster.EventDispatcher
	interruptChannel := make(chan error)

	eventDispatcher.DispatchUpdateWalFromLeaader(
		keyValuePair,
		targetIdx,
		&interruptChannel,
	)

	err = <-interruptChannel

	if err != nil {
		responseError(res, http.StatusBadRequest, err)
		return
	}

	responseOK(res, []byte{})
}

func HandleCommit(res http.ResponseWriter, req *http.Request) {

	tools.InfoLogger.Println(
		"커밋 메세지 도착!",
	)

	vars := mux.Vars(req)
	commitIdxString := vars["index"]
	commitIdx, err := strconv.ParseUint(
		commitIdxString,
		10,
		64,
	)

	if err != nil {

		tools.InfoLogger.Println(
			"커밋 메세지 에러! 인덱스 파싱 중 에러!",
		)
		responseError(res, http.StatusBadRequest, err)
		return
	}

	termString := req.Header.Get(cluster.TermHeader)
	if termString == "" {

		tools.InfoLogger.Println(
			"커밋 메세지 에러! Term 을 안보냄",
		)
		responseError(res, http.StatusBadRequest, err)
		return
	}

	term, err := strconv.ParseUint(
		termString,
		10,
		64,
	)

	if err != nil {
		responseError(res, http.StatusBadRequest, err)
		return
	}

	eventDispatcher := cluster.EventDispatcher
	interruptChannel := make(chan error)

	eventDispatcher.DispatchCommit(
		commitIdx,
		term,
		&interruptChannel,
	)

	err = <-interruptChannel
	if err != nil {
		responseError(res, http.StatusBadRequest, err)
		return
	}

	responseOK(res, []byte{})

}

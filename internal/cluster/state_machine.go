package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash_interface/internal/models"
	"hash_interface/internal/storage"
	"hash_interface/tools"
	"log"
	"net/http"
	"strings"
	"sync"
)

type Status string

const (
	Stopped   Status = "stopped"
	Follower  Status = "follower"
	Leader    Status = "leader"
	Candidate Status = "candidate"
)

type MsgType string

const (
	AppendEntry    MsgType = "appendEntry"
	UpdateEntry    MsgType = "updateEntry"
	VoteRequest    MsgType = "electionVote"
	ElectionResult MsgType = "electionResult"
	Error          MsgType = "error"
	EmptyMsg       MsgType = "empty"
	ToLeader       MsgType = "toLeader"
	Commit         MsgType = "commit"
	Heartbeat      MsgType = "heartbeat"
	UpdateFinished MsgType = "updateFinished"
)

type ClusterMsg struct {
	Msg               string
	NewLeader         string
	Value             string
	From              string
	Term              uint64
	IndexTime         uint64
	CommitIndex       uint64
	Err               error
	Type              MsgType
	IsAskedToBeUpdate bool
	IsElected         bool
	KeyValuePairs     []storage.KeyValuePair
	MetaData          map[string]interface{}
	InterruptChannel  *(chan error)
}

type StateMachine struct {
	Status Status

	IndexTime uint64

	CommitIndex uint64

	MetaDataLock *sync.Mutex

	LeaderAddress string

	Term uint64

	WriteAheadLog []storage.KeyValuePair

	WriteLock *sync.Mutex

	IsUpdatingFromLeader bool

	UpdateFromLeaderLock *sync.Mutex

	ScheduleChannel *(chan ClusterMsg)

	Cluster *ClusterInfo
}

var StateNode *StateMachine

const (
	FromLeader   = true
	FromFollower = false
)

func init() {
	if StateNode == nil {
		StateNode = &StateMachine{}

		err := StateNode.Init()
		if err != nil {
			log.Fatal(err.Error())
		}

		EventDispatcher = &Dispatcher{}
		EventDispatcher.Init(StateNode.ScheduleChannel)
	}
}

func (this *StateMachine) Init() error {

	this.Status = Stopped
	this.WriteLock = &sync.Mutex{}
	this.MetaDataLock = &sync.Mutex{}
	this.UpdateFromLeaderLock = &sync.Mutex{}
	this.IsUpdatingFromLeader = false
	this.WriteAheadLog = make([]storage.KeyValuePair, 100)
	this.WriteAheadLog[0] = storage.KeyValuePair{} // Dummy를 0번째 index에

	this.IndexTime = 0

	scheduleChannel := make(chan ClusterMsg)
	this.ScheduleChannel = &scheduleChannel

	this.Cluster = &ClusterInfo{}
	err := this.Cluster.Init()
	if err != nil {
		return err
	}

	return nil
}

func (this *StateMachine) Start(isStartPoint bool) {

	go func() {
		if isStartPoint {
			this.broadcastRaftStart()
		}
		this.StartEventLoop()
	}()
}

func (this *StateMachine) SendToLeader(key, value string) error {

	leader := this.GetLeader()

	this.MetaDataLock.Lock()
	idxTime := this.GetIndexTime(false)
	term := this.getTerm()
	this.MetaDataLock.Unlock()

	tools.InfoLogger.Printf(
		"리더 (%s) 에게 set key : %s, value : %s 전달",
		leader,
		key,
		value,
	)

	// 재시도 하지 않고 대기로 변경
	// 재시도 했다가 중복된 연산이 WAL에 쌓일까봐
	err := sendAppendWalMsg(
		leader,
		key,
		value,
		"",
		term,
		idxTime,
		FromFollower,
	)

	return err

}

func (this *StateMachine) GetLeader() string {
	return strings.TrimSpace(this.LeaderAddress)
}

func (this *StateMachine) IsInternalMsg(req *http.Request) bool {

	internalToken := req.Header.Get(InternalTokenHeader)

	if internalToken != "liverpool" {
		return true
	}

	return false
}

func (this *StateMachine) IsMyselfLeader() bool {
	return this.LeaderAddress == this.Cluster.curIpAddress
}

func (this *StateMachine) IsUpdateingFromLeader() bool {
	this.MetaDataLock.Lock()
	isUpdating := this.IsUpdatingFromLeader
	this.MetaDataLock.Unlock()

	return isUpdating
}

func (this *StateMachine) SetIsUpdateingFromLeader(isUpdating bool) {
	this.MetaDataLock.Lock()
	this.IsUpdatingFromLeader = isUpdating
	this.MetaDataLock.Unlock()
}

// MetaData Lock
//
func AskLeaderForUpdate(
	leader, origin string,
	startIdx uint64,
	resultChannel *(chan ClusterMsg),
) {

	if leader == "" {
		*resultChannel <- ClusterMsg{
			Type: UpdateFinished,
		}
		return
	}

	tools.InfoLogger.Printf(
		"리더 (%s) 에게 Write Ahead Log 업데이트 요청!",
		leader,
	)

	requestURI := fmt.Sprintf(
		"http://%s/api/v1/cluster/update",
		leader,
	)

	updateWalReq, err := http.NewRequest(
		http.MethodGet,
		requestURI,
		nil,
	)
	if err != nil {
		*resultChannel <- ClusterMsg{
			Type: UpdateFinished,
		}
		return
	}

	indexTimeString := fmt.Sprintf(
		"%d",
		startIdx,
	)

	updateWalReq.Header.Set(
		IndexTimeHeader,
		indexTimeString,
	)

	updateWalReq.Header.Set(
		OriginHeader,
		origin,
	)

	client := &http.Client{}

	// 리더가 Update용 HTTP Post request를 다 보낸 뒤에
	// 이 요청에 대한 응답이 올 것이다
	res, err := client.Do(updateWalReq)
	if err != nil {
		tools.ErrorLogger.Printf(
			"Write Ahead Log 업데이트 요청 실패 : %s",
			err.Error(),
		)
		*resultChannel <- ClusterMsg{
			Type: UpdateFinished,
		}
		return
	}

	if res.StatusCode >= 400 {

		tools.InfoLogger.Printf(
			"Write Ahead Log 업데이트 요청 실패",
		)

	}

	*resultChannel <- ClusterMsg{
		Type: UpdateFinished,
	}
}

func (this *StateMachine) GetWriteAheadLog() []storage.KeyValuePair {
	return this.WriteAheadLog
}

func (this *StateMachine) GetIndexTime(lockOption bool) uint64 {

	if lockOption {
		this.MetaDataLock.Lock()
		defer this.MetaDataLock.Unlock()
	}

	return this.IndexTime
}

func (this *StateMachine) setIndexTime(newIndexTime uint64) {
	this.IndexTime = newIndexTime
}

func (this *StateMachine) setTerm(term uint64) {
	this.Term = term
}

func (this *StateMachine) getTerm() uint64 {
	return this.Term
}

func (this *StateMachine) Propagate(req *http.Request) {

	var requestedData models.DataRequestContainer
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&requestedData); err != nil {

	}

	client := &http.Client{}

	for _, eachNode := range this.Cluster.nodeAddressList {

		go func(target string) {

			requestURI := fmt.Sprintf(
				"http://%s/api/v1/hash/data",
				target,
			)

			encodedData, err := json.Marshal(requestedData)
			if err != nil {

				return
			}

			requestBody := bytes.NewBuffer(encodedData)

			setRequest, err := http.NewRequest(
				http.MethodPost,
				requestURI,
				requestBody,
			)

			if err != nil {
				return
			}

			indexTime := this.GetIndexTime(true)
			indexTimeString := fmt.Sprintf(
				"%d",
				indexTime+1,
			)

			setRequest.Header.Set(
				IndexTimeHeader,
				indexTimeString,
			)

			setRequest.Header.Set(
				InternalTokenHeader,
				"liverpool",
			)

			client.Do(setRequest)

		}(eachNode)

	}

}

func (this *StateMachine) DidAskToLeader(req *http.Request) bool {

	startIdxTimeString := req.Header.Get(StartIndexTimeHeader)

	return startIdxTimeString != ""
}

func (this *StateMachine) ShouldParticipate(term uint64) bool {

	if this.Term < term {
		return true
	}

	return false
}

func (this *StateMachine) StartElection() {

	nextTerm := this.Term + 1

	this.MetaDataLock.Lock()
	this.setNewLeader("")
	this.setTerm(nextTerm)
	this.MetaDataLock.Unlock()

	tools.InfoLogger.Printf(
		"제 %d 대 선거 시작 : 출마한 노드(%s)",
		nextTerm,
		this.Cluster.curIpAddress,
	)

	go this.askForVote(nextTerm)

}

func (this *StateMachine) HasNoLeader() bool {

	this.MetaDataLock.Lock()
	defer this.MetaDataLock.Unlock()

	if this.GetLeader() == "" {
		return true
	}

	return false
}

func (this *StateMachine) WaitForNewLeader() {

	// Lock 대신 Busy Waiting
	for {
		if this.GetLeader() != "" {
			return
		}
	}
}

func (this *StateMachine) IsRunning() bool {
	return this.Status != Stopped
}

func (this *StateMachine) setNewLeader(leaderAddress string) {
	this.LeaderAddress = strings.TrimSpace(leaderAddress)
}

func (this *StateMachine) becomeCandidate() {
	this.Status = Candidate
}

func (this *StateMachine) becomeLeader() {
	this.Status = Leader
}

func (this *StateMachine) becomeFollower() {
	this.Status = Follower
}

func (this *StateMachine) GetStatus() Status {
	return this.Status
}

func (this *StateMachine) setCommitIndex(index uint64) {
	this.CommitIndex = index
}

func (this *StateMachine) getCommitIdx() uint64 {
	return this.CommitIndex
}

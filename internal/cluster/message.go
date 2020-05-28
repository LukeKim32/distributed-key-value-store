package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash_interface/internal/models"
	"hash_interface/internal/storage"
	"hash_interface/tools"
	"net/http"
	"time"
)

const (
	TermHeader          = "term"
	LeaderHeader        = "leader"
	InternalTokenHeader = "internalToken"
	OriginHeader        = "origin"
	PassToStoreHeader   = "freePass"
	IndexTimeHeader     = "indexTime"
	CommitIndexHeader   = "commitIndex"

	// startIndexTimeHeader : WAL 업데이트를 요청한 Follower의 Index Time
	StartIndexTimeHeader = "startIndexTime"
)

func (this *StateMachine) sendWalUpdateMsg(
	targetFollower string,
	keyValuePair storage.KeyValuePair,
	targetIdx uint64,
) error {

	requestURI := fmt.Sprintf(
		"http://%s/api/v1/cluster/update",
		targetFollower,
	)

	requestData := models.DataRequestContainer{}
	requestData.Data = append(
		requestData.Data,
		keyValuePair,
	)

	encodedData, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	requestBody := bytes.NewBuffer(encodedData)

	walUpdateReq, err := http.NewRequest(
		http.MethodPost,
		requestURI,
		requestBody,
	)
	if err != nil {
		return err
	}

	targetIdxString := fmt.Sprintf(
		"%d",
		targetIdx,
	)

	walUpdateReq.Header.Set(
		IndexTimeHeader,
		targetIdxString,
	)
	walUpdateReq.Header.Set(
		InternalTokenHeader,
		"liverpool",
	)

	client := &http.Client{}
	res, err := client.Do(walUpdateReq)
	if err != nil {
		tools.InfoLogger.Println(
			"리더 => 팔로워 오류",
		)
		return err
	}

	if res.StatusCode >= 400 {
		tools.InfoLogger.Println(
			"리더 => 팔로워 오류",
		)
		return err
	}

	tools.InfoLogger.Printf(
		"리더 => 팔로워 - (키,밸류) : (%s,%s), 타겟인덱스 : %d",
		keyValuePair.Key,
		keyValuePair.Value,
		targetIdx,
	)

	return nil
}

type Register struct {
	Address string
}

func (this *StateMachine) SendRegisterMsg(
	targetHost, newHost string,
	handshake, startPoint bool,
) error {

	// 등록 상대 노드에도 등록 요청
	requestURI := fmt.Sprintf(
		"http://%s/api/v1/cluster?handshake=%v&startPoint=%v",
		targetHost,
		handshake,
		startPoint,
	)

	tools.InfoLogger.Printf(
		"목표 Host : %s, 등록 요청할 Host : %s ",
		targetHost,
		newHost,
	)

	requestData := Register{
		Address: newHost,
	}

	encodedData, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	requestBody := bytes.NewBuffer(encodedData)

	registerReq, err := http.NewRequest(
		http.MethodPost,
		requestURI,
		requestBody,
	)
	if err != nil {
		return err
	}

	client := &http.Client{}
	res, err := client.Do(registerReq)
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("클러스터 노드 등록 실패")
	}

	return nil
}

func (this *StateMachine) broadcastRaftStart() {
	for _, eachNode := range this.Cluster.nodeAddressList {
		go sendRaftStartMsg(eachNode)
	}
}

func sendRaftStartMsg(targetAddress string) {

	// 등록 상대 노드에도 등록 요청
	requestURI := fmt.Sprintf(
		"http://%s/api/v1/cluster?startPoint=false",
		targetAddress,
	)

	raftStartReq, _ := http.NewRequest(
		http.MethodPut,
		requestURI,
		nil,
	)

	client := &http.Client{}
	client.Do(raftStartReq)

}

func (this *StateMachine) askForVote(term uint64) {

	for _, eachNode := range this.Cluster.nodeAddressList {
		go func(resultChannel *(chan ClusterMsg), targetNode string) {

			sendVoteMsg(targetNode, this.Cluster.curIpAddress, term, resultChannel)

		}(this.ScheduleChannel, eachNode)
	}
}

func sendVoteMsg(
	targetAddress, selfAddress string,
	term uint64,
	resultChannel *chan ClusterMsg,
) {

	// start := time.Now()

	// 등록 상대 노드에도 등록 요청
	requestURI := fmt.Sprintf(
		"http://%s/api/v1/cluster/election",
		targetAddress,
	)

	voteEncourageReq, err := http.NewRequest(
		http.MethodGet,
		requestURI,
		nil,
	)
	if err != nil {
		(*resultChannel) <- ClusterMsg{
			Type: Error,
		}
		return
	}

	termString := fmt.Sprintf(
		"%d",
		term,
	)

	voteEncourageReq.Header.Set(
		TermHeader,
		termString,
	)

	client := &http.Client{}
	res, err := client.Do(voteEncourageReq)

	if err != nil {
		tools.InfoLogger.Printf(
			"제 %d 대 선거 : 노드(%s)의 투표 결과 : 미투표",
			term,
			targetAddress,
		)

		(*resultChannel) <- ClusterMsg{
			Type: Error,
		}
		return
	}

	// elapsed := time.Since(start)

	if res.StatusCode >= 400 {

		tools.InfoLogger.Printf(
			"제 %d 대 선거 : 노드(%s)의 투표 결과 : 미투표",
			term,
			targetAddress,
		)

		(*resultChannel) <- ClusterMsg{
			Type: Error,
		}
		return
	}

	tools.InfoLogger.Printf(
		"제 %d 대 선거 : 노드(%s)의 투표 결과 : 투표! 감사합니다",
		term,
		targetAddress,
	)

	(*resultChannel) <- ClusterMsg{
		Type: ElectionResult,
		Term: term,
	}
	return
}

// MetaData Lock 없음
//
func (this *StateMachine) broadcastElected() {

	curTerm := this.getTerm()

	tools.InfoLogger.Printf(
		"제 %d 대 선거 결과, 제가 Leader입니다!",
		curTerm,
	)

	for _, eachNode := range this.Cluster.nodeAddressList {
		go sendElectedMsg(
			eachNode,
			this.Cluster.curIpAddress,
			curTerm,
		)
	}
}

type HeartbeatMsg struct {
	Term   uint64
	Leader string
}

func sendElectedMsg(targetAddress, newLeader string, term uint64) {

	// 등록 상대 노드에도 등록 요청
	requestURI := fmt.Sprintf(
		"http://%s/api/v1/cluster/heartbeat",
		targetAddress,
	)

	newLeaderElectedReq, _ := http.NewRequest(
		http.MethodGet,
		requestURI,
		nil,
	)

	termString := fmt.Sprintf(
		"%d",
		term,
	)

	newLeaderElectedReq.Header.Set(
		TermHeader,
		termString,
	)

	newLeaderElectedReq.Header.Set(
		LeaderHeader,
		newLeader,
	)

	newLeaderElectedReq.Header.Set(
		InternalTokenHeader,
		"liverpool",
	)

	client := &http.Client{}
	_, err := client.Do(newLeaderElectedReq)

	if err != nil {
		tools.InfoLogger.Printf(
			"다른 노드들에게 전송한 당선 발표 실패 %s",
			err.Error(),
		)
		return
	}

	tools.InfoLogger.Printf(
		"팔로워(%s) 에게 자신의 당선 발표!",
		targetAddress,
	)

	return
}

func (this *StateMachine) broadcastAppendWal(key, value string) error {

	resultChannel := make(chan error)

	this.MetaDataLock.Lock()
	curTerm := this.getTerm()
	leaderIdx := this.GetIndexTime(false)
	this.MetaDataLock.Unlock()

	tools.InfoLogger.Printf(
		"AppendEntry 다른 친구들에게 전부! 전달 : key : %s, value : %s, 현재 term : %d, 리더 Index : %d",
		key,
		value,
		curTerm,
		leaderIdx,
	)

	for _, eachNode := range this.Cluster.nodeAddressList {
		go func(resultChannel *(chan error), target string) {

			err := sendAppendWalMsg(
				target,
				key,
				value,
				this.Cluster.curIpAddress,
				curTerm,
				leaderIdx,
				FromLeader,
			)

			*resultChannel <- err

		}(&resultChannel, eachNode)
	}

	timeoutChannel := time.After(MaxTimeout)

	appendWalCompleteFollowerCnt := 1
	majority := (len(this.Cluster.nodeAddressList) / 2) + 1
	msgCnt := 0

	for {
		select {
		case err := <-resultChannel:

			msgCnt += 1

			if err == nil {
				appendWalCompleteFollowerCnt += 1
			} else {
				tools.InfoLogger.Printf(
					"리더가 AppendWal 보냈는데 에러 응답받음 %s",
					err.Error(),
				)
			}

			if appendWalCompleteFollowerCnt >= majority {
				return nil
			}

			if msgCnt == len(this.Cluster.nodeAddressList) {
				if appendWalCompleteFollowerCnt >= majority {
					return nil
				} else {
					return fmt.Errorf(
						"모든 노드로부터 메세지 다 왔는데 고반수 이하 득표, Commit하면 안된다",
					)
				}
			}

		case <-timeoutChannel:
			if appendWalCompleteFollowerCnt >= majority {
				return nil
			} else {
				return fmt.Errorf(
					"AppendWal 보내고 타임아웃 이후 과반수 이하 득표, Commit하면 안된다",
				)
			}
		}
	}
}

func sendAppendWalMsg(
	targetAddress, key, value, leader string,
	curTerm, indexTime uint64,
	isFromLeader bool,
) error {

	// 등록 상대 노드에도 등록 요청
	requestURI := fmt.Sprintf(
		"http://%s/api/v1/cluster/wal",
		targetAddress,
	)

	if isFromLeader {
		requestURI += "/leader"
	}

	requestData := models.DataRequestContainer{}
	requestData.Data = append(
		requestData.Data,
		storage.KeyValuePair{
			Key:   key,
			Value: value,
		},
	)

	encodedData, _ := json.Marshal(requestData)

	requestBody := bytes.NewBuffer(encodedData)

	appendWalReq, _ := http.NewRequest(
		http.MethodPost,
		requestURI,
		requestBody,
	)

	appendWalReq.Header.Set(
		InternalTokenHeader,
		"liverpool",
	)

	termString := fmt.Sprintf(
		"%d",
		curTerm,
	)

	appendWalReq.Header.Set(
		TermHeader,
		termString,
	)

	indexTimeString := fmt.Sprintf(
		"%d",
		indexTime,
	)

	appendWalReq.Header.Set(
		IndexTimeHeader,
		indexTimeString,
	)

	if isFromLeader {
		appendWalReq.Header.Set(
			LeaderHeader,
			leader,
		)
	}

	client := &http.Client{}
	res, err := client.Do(appendWalReq)
	if err != nil {
		tools.InfoLogger.Printf(
			"리더가 AppendWal 보내는데 에러 : %s",
			err.Error(),
		)
		return err
	}

	if res.StatusCode >= 400 {

		tools.InfoLogger.Printf(
			"리더가 팔로워(%s)에게 AppendWal 보내는데 에러2",
			targetAddress,
		)

		return fmt.Errorf("Update Error")
	}

	return nil
}

func (this *StateMachine) broadcastHeartbeat() {

	this.MetaDataLock.Lock()
	term := this.getTerm()
	idxTime := this.GetIndexTime(false)
	commitIdx := this.getCommitIdx()
	this.MetaDataLock.Unlock()

	for _, eachNode := range this.Cluster.nodeAddressList {
		go sendHearbeat(
			eachNode,
			this.Cluster.curIpAddress,
			term,
			idxTime,
			commitIdx,
		)
	}

}

func sendHearbeat(
	targetAddress, leader string,
	term, idxTime, commitIdx uint64,
) {
	// 등록 상대 노드에도 등록 요청
	requestURI := fmt.Sprintf(
		"http://%s/api/v1/cluster/heartbeat",
		targetAddress,
	)

	heartbeatReq, _ := http.NewRequest(
		http.MethodGet,
		requestURI,
		nil,
	)

	heartbeatReq.Header.Set(
		InternalTokenHeader,
		"liverpool",
	)

	leaderTerm := fmt.Sprintf(
		"%d",
		term,
	)
	heartbeatReq.Header.Set(
		TermHeader,
		leaderTerm,
	)

	leaderIdx := fmt.Sprintf(
		"%d",
		idxTime,
	)

	heartbeatReq.Header.Set(
		IndexTimeHeader,
		leaderIdx,
	)

	commitIdxString := fmt.Sprintf(
		"%d",
		commitIdx,
	)

	heartbeatReq.Header.Set(
		CommitIndexHeader,
		commitIdxString,
	)

	heartbeatReq.Header.Set(
		LeaderHeader,
		leader,
	)

	client := &http.Client{}
	client.Do(heartbeatReq)

}

func (this *StateMachine) broadcastCommit() {

	this.MetaDataLock.Lock()
	defer this.MetaDataLock.Unlock()

	commitIdx := this.getCommitIdx()

	term := this.getTerm()
	leaderIdxTime := this.GetIndexTime(false)

	tools.InfoLogger.Printf(
		"커밋 %d 메세지 팔로워들에게 전송!",
		commitIdx,
	)

	for _, eachNode := range this.Cluster.nodeAddressList {
		go sendCommitMsg(
			eachNode,
			commitIdx,
			term,
			leaderIdxTime,
		)
	}
}

func sendCommitMsg(targetNode string, commitIdx, term, leaderIdxTime uint64) {

	requestURI := fmt.Sprintf(
		"http://%s/api/v1/cluster/commit/%d",
		targetNode,
		commitIdx,
	)

	commitReq, _ := http.NewRequest(
		http.MethodGet,
		requestURI,
		nil,
	)

	commitReq.Header.Set(
		InternalTokenHeader,
		"liverpool",
	)

	leaderTerm := fmt.Sprintf(
		"%d",
		term,
	)

	commitReq.Header.Set(
		TermHeader,
		leaderTerm,
	)

	leaderIdx := fmt.Sprintf(
		"%d",
		leaderIdxTime,
	)

	commitReq.Header.Set(
		IndexTimeHeader,
		leaderIdx,
	)

	commitIdxString := fmt.Sprintf(
		"%d",
		commitIdx,
	)

	commitReq.Header.Set(
		CommitIndexHeader,
		commitIdxString,
	)

	client := &http.Client{}
	res, err := client.Do(commitReq)
	if err != nil {
		tools.InfoLogger.Printf(
			"리더가 커밋메세지 날린게 에러 발생! : %s",
			err.Error(),
		)
		return
	}

	if res.StatusCode >= 400 {
		tools.InfoLogger.Printf(
			"리더가 커밋메세지 날린게 에러 발생2 ",
		)
		return
	}
}

func saveData(keyValuePair storage.KeyValuePair) error {

	// 컨테이너는 독립된 가상 네트워크로 구성되어있으므로
	// 컨테이너 기준 로컬호스트는 컨테이너가 둘러쌓여진 가상네트워크 공간이다
	// 따라서 컨테이너가 떠있는 포트로 전달해야한다!
	requestURI := fmt.Sprintf(
		"http://localhost:8888/api/v1/hash/data",
	)

	requestData := models.DataRequestContainer{}
	requestData.Data = append(
		requestData.Data,
		keyValuePair,
	)

	encodedData, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	requestBody := bytes.NewBuffer(encodedData)

	saveReq, err := http.NewRequest(
		http.MethodPost,
		requestURI,
		requestBody,
	)

	saveReq.Header.Set(
		PassToStoreHeader,
		"true",
	)

	client := &http.Client{}
	res, err := client.Do(saveReq)
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("데이터 저장 실패")
	}

	return nil
}

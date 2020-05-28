package storage

import (
	"fmt"
	msg "hash_interface/internal/storage/message"
	"hash_interface/tools"
	"strings"
	"sync"

	"github.com/gomodule/redigo/redis"
)

const (
	MasterRole = "master"
	SlaveRole  = "slave"
)

type RedisClient struct {
	Connection redis.Conn `json:"-"`
	Address    string     `json:"address"`
	Role       string     `json:"role"`
}

var redisMasterClients []RedisClient
var redisSlaveClients []RedisClient

// masterSlaveMap : 마스터 주소 -> 슬레이브 노드
var masterSlaveMap map[string]RedisClient

// slaveMasterMap : 슬레이브 주소 -> 마스터 노드
var slaveMasterMap map[string]RedisClient

// redisMutexMap : IP주소 -> 뮤텍스, 각 마스터-슬레이브 그룹 별 요청 동기화용
// used for sync of request to each Redis <Master-Slave> Set With key of "Address"
var redisMutexMap map[string]*sync.Mutex

// addClientMutex : 레디스 클라이언트 추가 동기화용
var addClientMutex *sync.Mutex

/****************************************
 *
 *
 *        Master Client Method
 *
 *
 ****************************************/

// handleIfDead : 인스턴스의 생존여부를 확인하고, failover 발생 시 처리
//  1. masterClient 인스턴스가 살아있는지 확인
//  2. 죽었을 경우
//   1) 매핑된 Slave를 새로운 마스터로 승격
//   2) 죽은 masterClient는 재시작
//  3. 새로운 마스터 승격이 실패할 경우 (Slave 죽은 것으로 판단) 남은 Master Client들에게 해쉬슬롯 재분배
//
func (masterClient *RedisClient) handleIfDead() error {

	if masterClient.Role != MasterRole {
		return fmt.Errorf(msg.NotAllowedIfNotMaster)
	}

	// 모니터 서버에게 생존 확인/투표 요청
	numberOfTotalVotes := len(monitorClient.ServerAddressList) + 1
	votes, err := monitorClient.ask(*masterClient, IsAlive)
	if err != nil {
		return err
	}

	// 호스트 인터페이스 서버의 생존 확인/투표
	hostPingResult, hostPingErr := redis.String(masterClient.Connection.Do("PING"))
	if strings.Contains(hostPingResult, "PONG") {
		votes++
	}

	// tools.InfoLogger.Printf(
	// 	msg.FailOverVoteResult,
	// 	masterClient.Address,
	// 	votes,
	// 	numberOfTotalVotes,
	// )

	// 과반수가 살아있다고 판단
	if votes > (numberOfTotalVotes / 2) {

		// 호스트 연결 에러시, 재연결 시도
		if hostPingErr != nil {

			if err := masterClient.TryReconnect(); err != nil {
				tools.ErrorLogger.Printf(
					msg.ConnectionFailure,
					masterClient.Address,
					err.Error(),
				)
				return err
			}
		}

		masterClient.checkSlaveAlive()

		return nil
	}

	// 과반수 이상 죽었다고 판단한 경우

	//tools.InfoLogger.Printf(msg.PromotinSlaveStart, masterClient.Address)

	slaveClient, isSet := masterSlaveMap[masterClient.Address]
	if isSet == false {
		return fmt.Errorf(msg.MasterSlaveMapNotInit)
	}

	// 슬레이브를 새로운 마스터로 승격
	if err := slaveClient.promoteToMaster(); err != nil {

		if err.Error() == msg.BothMasterSlaveDead {

			// 마스터 - 슬레이브 모두 죽은 경우
			// 해쉬 슬롯 재분배 후, 마스터-슬레이브의 모든 데이터 및 설정 삭제
			if err := hashSlot.distributeFrom(masterClient); err != nil {
				return err
			}
			return nil
		}
		return err
	}

	//tools.InfoLogger.Printf(msg.PromotionSuccess, masterClient.Address)

	// 새로운 마스터로 승격 성공
	// 죽은 기존 마스터는 재시작 (using docker API)
	err = docker.restartRedisContainer(masterClient.Address)

	// 컨테이너 재시작이 성공한 경우에만 새로운 마스터의 슬레이브로 연결 시도
	if err == nil {

		masterClient.connectToMaster(&slaveClient)

	} else {
		tools.ErrorLogger.Println(err)
	}

	// tools.InfoLogger.Printf(
	// 	msg.SlaveAsNewMaster,
	// 	slaveMasterMap[masterClient.Address].Address,
	// )

	return nil
}

// getSlave : 자신의 슬레이브가 살아있을 경우 반환, 죽어있을 경우 에러 반환
// To-Do : 1-1 매핑이 아닌, 슬레이브가 여러 대일 경우 처리
//
func (masterClient RedisClient) getSlave() (RedisClient, error) {

	slaveClient, isSet := masterSlaveMap[masterClient.Address]
	if isSet == false {
		return RedisClient{}, fmt.Errorf("")
	}

	// 모니터 서버에 슬레이브 생존 여부 확인
	numberOfTotalVotes := len(monitorClient.ServerAddressList) + 1
	votes, err := monitorClient.ask(slaveClient, IsAlive)
	if err != nil {
		return RedisClient{}, fmt.Errorf("")
	}

	// 호스트 인터페이스 서버의 생존 확인/투표
	hostPingResult, _ := redis.String(slaveClient.Connection.Do("PING"))
	if strings.Contains(hostPingResult, "PONG") {
		votes++
	}

	// tools.InfoLogger.Printf(
	// 	msg.FailOverVoteResult,
	// 	slaveClient.Address,
	// 	votes,
	// 	numberOfTotalVotes,
	// )

	// 과반수 이상이 죽었다고 판단
	if votes < (numberOfTotalVotes / 2) {
		err := fmt.Errorf(msg.VoteResultSlaveDead, slaveClient.Address)
		return RedisClient{}, err
	}

	return slaveClient, nil
}

// handleIfDeadWithLock : Monitor 루틴에 사용되는 메소드
// handleIfDead() 의 확장으로 Lock을 이용한다
//
func (masterClient RedisClient) handleIfDeadWithLock(errorChannel chan error) {

	redisMutexMap[masterClient.Address].Lock()
	defer redisMutexMap[masterClient.Address].Unlock()

	// Redis Node can be discarded from Master nodes if redistribute happens
	if masterClient.Role != MasterRole {
		return
	}

	if err := masterClient.handleIfDead(); err != nil {
		errorChannel <- err
		return
	}
}

// checkSlaveAlive : masterClient 인스턴스의 slave의 생존 여부 확인 & 죽었을 시 재시작
//
func (masterClient *RedisClient) checkSlaveAlive() {

	slaveClient, isSet := masterSlaveMap[masterClient.Address]
	if isSet == false {
		return
	}

	// tools.InfoLogger.Printf(msg.StartSlaveAliveCheck, masterClient.Address)

	numberOfTotalVotes := len(monitorClient.ServerAddressList) + 1
	votes, err := monitorClient.ask(slaveClient, IsAlive)
	if err != nil {
		return
	}

	// 호스트 인터페이스 서버의 생존 확인/투표
	hostPingResult, _ := redis.String(slaveClient.Connection.Do("PING"))
	if strings.Contains(hostPingResult, "PONG") {
		votes++
	}

	// tools.InfoLogger.Printf(
	// 	msg.FailOverVoteResult,
	// 	slaveClient.Address,
	// 	votes,
	// 	numberOfTotalVotes,
	// )

	// 과반수가 살아있다고 판단
	if votes > (numberOfTotalVotes / 2) {
		//tools.InfoLogger.Printf(msg.SlaveIsAlive, slaveClient.Address)
		return
	}

	// tools.InfoLogger.Printf(msg.VoteResultSlaveDead, slaveClient.Address)

	// 죽었을 경우
	slaveClient.removeDataLogFile()
	slaveClient.RemoveFromList()

	err = docker.restartRedisContainer(slaveClient.Address)

	// 컨테이너 재시작이 성공한 경우에만 새로운 마스터의 슬레이브로 연결 시도
	if err == nil {
		slaveClient.connectToMaster(masterClient)
	}
}

// cleanUpMemory : masterClient 인스턴스와 이에 매핑된 Slave Client 의 메모리 해제
// Garbace Collect
// To-Do : 여러 Slave Client들에 대한 처리
//
func (masterClient RedisClient) cleanUpMemory() error {

	clientHashRangeMap[masterClient.Address] = nil
	delete(clientHashRangeMap, masterClient.Address)

	slaveClient := masterSlaveMap[masterClient.Address]

	if err := masterClient.RemoveFromList(); err != nil {
		return err
	}

	if err := slaveClient.RemoveFromList(); err != nil {
		return err
	}

	if _, err := monitorClient.ask(masterClient, EndConnect); err != nil {
		return err
	}

	if _, err := monitorClient.ask(slaveClient, EndConnect); err != nil {
		return err
	}

	delete(MasterSlaveChannelMap, masterClient.Address)
	delete(masterSlaveMap, masterClient.Address)
	delete(slaveMasterMap, slaveClient.Address)

	return nil
}

/****************************************
 *
 *
 *        Slave Client Method
 *
 *
 ****************************************/

// promoteToMaster : slaveClient 인스턴스 생존 여부 확인, 새로운 마스터로 승격
//  1. 기존 마스터가 담당하던 해쉬 슬롯 할당
//  2. 마스터, 슬레이브 관련 변수 초기화
//
func (slaveClient *RedisClient) promoteToMaster() error {

	// 슬레이브가 살아있는지 확인
	numberOfTotalVotes := len(monitorClient.ServerAddressList) + 1
	votes, err := monitorClient.ask(*slaveClient, IsAlive)
	if err != nil {
		return err
	}

	// 호스트 인터페이스 서버의 생존 확인/투표
	hostPingResult, _ := redis.String(slaveClient.Connection.Do("PING"))
	if strings.Contains(hostPingResult, "PONG") {
		votes++
	}

	// tools.InfoLogger.Printf(
	// 	msg.FailOverVoteResult,
	// 	slaveClient.Address,
	// 	votes,
	// 	numberOfTotalVotes,
	// )

	// 과반수 이상이 죽었다고 판단한 경우
	if votes <= (numberOfTotalVotes / 2) {
		return fmt.Errorf(msg.BothMasterSlaveDead)
	}

	//tools.InfoLogger.Printf(msg.PromotingSlaveNode, slaveClient.Address)

	masterClient, isSet := slaveMasterMap[slaveClient.Address]
	if isSet == false {
		return fmt.Errorf(msg.MasterSlaveMapNotInit)
	}

	// 새로운 마스터로 승격 시작
	// 1. 기존 마스터와, 새로운 마스터 설정 초기화
	if err := slaveClient.setUpMasterConfig(); err != nil {
		return err
	}

	// 2. 기존 마스터의 해쉬 슬롯을 이어 받음
	for _, eachHashRangeOfClient := range clientHashRangeMap[masterClient.Address] {
		hashSlotStart := eachHashRangeOfClient.startIndex
		hashSlotEnd := eachHashRangeOfClient.endIndex

		// 2-1. 해쉬 맵 업데이트
		hashSlot.assign(slaveClient, hashSlotStart, hashSlotEnd)

		newHashRange := HashRange{
			startIndex: hashSlotStart,
			endIndex:   hashSlotEnd,
		}

		// 2-2. 담당하는 해쉬 슬롯 범위에 추가
		clientHashRangeMap[slaveClient.Address] = append(
			clientHashRangeMap[slaveClient.Address],
			newHashRange,
		)
	}

	// 기존 마스터의 해쉬 슬롯 범위 제거 (Garbage Collect)
	clientHashRangeMap[masterClient.Address] = nil
	delete(clientHashRangeMap, masterClient.Address)

	return nil
}

// setUpMasterConfig : slaveClient 인스턴스를 마스터 Client의 설정 추가, 기존 마스터 Client의 설정 삭제
//
func (slaveClient *RedisClient) setUpMasterConfig() error {

	masterClient, isSet := slaveMasterMap[slaveClient.Address]
	if isSet == false {
		return fmt.Errorf(msg.MasterSlaveMapNotInit)
	}

	if err := masterClient.removeDataLogFile(); err != nil {
		return err
	}

	if err := slaveClient.RemoveFromList(); err != nil {
		return err
	}

	if err := masterClient.RemoveFromList(); err != nil {
		return err
	}

	slaveClient.Role = MasterRole

	redisMasterClients = append(redisMasterClients, *slaveClient)

	delete(masterSlaveMap, masterClient.Address)
	delete(slaveMasterMap, slaveClient.Address)

	return nil
}

/****************************************
 *
 *
 *        General Client Method
 *
 *
 ****************************************/

// RemoveFromList : 인스턴스의 Role을 확인, 해당 리스트에서 삭제
//
func (redisClient RedisClient) RemoveFromList() error {

	//tools.InfoLogger.Printf(msg.SlaveNodeInfo, redisClient.Role, redisClient.Address)

	switch redisClient.Role {
	case MasterRole:
		for i, eachClient := range redisMasterClients {
			if eachClient.Address == redisClient.Address {

				redisMasterClients[i] = redisMasterClients[len(redisMasterClients)-1]
				redisMasterClients[len(redisMasterClients)-1] = RedisClient{}

				redisMasterClients = redisMasterClients[:len(redisMasterClients)-1]

				return nil
			}
		}
	case SlaveRole:
		for i, eachClient := range redisSlaveClients {
			if eachClient.Address == redisClient.Address {

				redisSlaveClients[i] = redisSlaveClients[len(redisSlaveClients)-1]
				redisSlaveClients[len(redisSlaveClients)-1] = RedisClient{}

				redisSlaveClients = redisSlaveClients[:len(redisSlaveClients)-1]

				return nil
			}
		}
	default:
		return fmt.Errorf(msg.RedisRoleNotInit, redisClient.Address)
	}

	return fmt.Errorf(msg.NoClientInList, redisClient.Address, redisClient.Role)
}

// isAlreadyExist : redisClient 인스턴스의 주소를 바탕으로 등록된 마스터, 슬레이브 리스트에 존재하는 지 확인
//
func (redisClient RedisClient) isAlreadyExist() bool {

	for _, eachClient := range redisMasterClients {
		if eachClient.Address == redisClient.Address {
			return true
		}
	}

	for _, eachClient := range redisSlaveClients {
		if eachClient.Address == redisClient.Address {
			return true
		}
	}

	return false
}

// GetMasterClients : 현재 모든 마스터 클라이언트를 리턴
func GetMasterClients() []RedisClient {
	return redisMasterClients
}

// GetSlaveClients : 현재 모든 슬레이브 클라이언트을 리턴
func GetSlaveClients() []RedisClient {
	return redisSlaveClients
}

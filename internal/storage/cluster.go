package storage

import (
	"fmt"
	msg "hash_interface/internal/storage/message"
	"hash_interface/tools"
	"time"
)

// GetRedisClient : 해쉬 슬롯의 @hashSlotIndex 번째 인덱스를 담당하는 Redis Client 반환
/* Check Process :
 * 1) Check Hash Mapped Redis Client First (Master Node)
 * 2) Get Votes From Monitor Servers
 * 3-1) If Votes are bigger than half, Re-setup connection and return
 * 3-2) If Votes are smaller than half, Promote It's Slave Client to New Master
 */
func GetRedisClient(hashSlotIndex uint16) (RedisClient, error) {

	tempClient := hashSlot.get(hashSlotIndex)

	redisMutexMap[tempClient.Address].Lock()

	// HashSlot이 handleIfDead() 재분배되어도, 이전 값 유지
	defer redisMutexMap[tempClient.Address].Unlock()

	start := time.Now()
	defer tools.InfoLogger.Printf(msg.FunctionExecutionTime, time.Since(start))

	// 반환 전, redisClient의 생존여부 확인/처리
	if err := tempClient.handleIfDead(); err != nil {
		return RedisClient{}, err
	}

	targetClient := hashSlot.get(hashSlotIndex)

	//tools.InfoLogger.Printf(msg.RedisNodeSelected, targetClient.Address)

	// handleIfDead에 의해 해쉬 슬롯이 바뀔 수도 있다.
	return targetClient, nil
}

func GetMasterWithAddress(address string) (*RedisClient, error) {

	if len(redisMasterClients) == 0 {
		return &RedisClient{}, fmt.Errorf(msg.NotAnyRedisSetUpYet)
	}

	for i, eachClient := range redisMasterClients {
		if eachClient.Address == address {
			//tools.InfoLogger.Println()
			return &redisMasterClients[i], nil
		}
	}

	tools.ErrorLogger.Println(msg.MasterFound)
	return &RedisClient{}, fmt.Errorf(msg.NoMatchingResponseNode)
}

func GetSlaveClientWithAddress(address string) (*RedisClient, error) {

	if len(redisSlaveClients) == 0 {
		return &RedisClient{}, fmt.Errorf(msg.NotAnyRedisSetUpYet)
	}

	for i, eachClient := range redisSlaveClients {
		if eachClient.Address == address {
			//tools.InfoLogger.Println(msg.SlaveFound)
			return &redisSlaveClients[i], nil
		}
	}

	tools.ErrorLogger.Println(msg.SlaveNotFound)
	return &RedisClient{}, fmt.Errorf(msg.NoMatchingResponseNode)
}

func swapMasterSlaveConfigs(masterNode *RedisClient, slaveNode *RedisClient) error {

	if err := slaveNode.RemoveFromList(); err != nil {
		return err
	}

	if err := masterNode.RemoveFromList(); err != nil {
		return err
	}

	slaveNode.Role = MasterRole
	masterNode.Role = SlaveRole

	redisSlaveClients = append(redisSlaveClients, *masterNode)
	redisMasterClients = append(redisMasterClients, *slaveNode)

	delete(masterSlaveMap, masterNode.Address)
	masterSlaveMap[slaveNode.Address] = *masterNode

	delete(slaveMasterMap, slaveNode.Address)
	slaveMasterMap[masterNode.Address] = *slaveNode

	return nil
}

func checkRedisClientSetup() error {

	if len(redisMasterClients) == 0 {
		return fmt.Errorf(msg.RedisMasterNotSetUpYet)
	}

	if len(redisSlaveClients) == 0 {
		return fmt.Errorf(msg.RedisSlaveNotSetUpYet)
	}

	return nil
}

func AddNewMaster(newMasterAddress string) error {

	addClientMutex.Lock()
	defer addClientMutex.Unlock()

	err := NodeConnectionSetup([]string{newMasterAddress}, Default)
	if err != nil {
		tools.ErrorLogger.Printf(
			"AddNewMaster() : %s",
			err.Error(),
		)
		return err
	}

	newMaster, err := GetMasterWithAddress(newMasterAddress)
	if err != nil {
		tools.ErrorLogger.Printf(msg.NewMasterNotFound)
		return err
	}

	// 모니터 서버에도 등록 요청
	if _, err := monitorClient.ask(*newMaster, NewConnect); err != nil {
		tools.ErrorLogger.Printf(msg.MonitorRegisterFail)
		return err
	}

	if ok := hashSlot.distributeTo(newMaster); ok != true {
		return fmt.Errorf(msg.DistributeToFail, newMaster.Address)
	}

	if err := newMaster.createDataLogFile(); err != nil {
		return err
	}

	if err := newMaster.reshardData(); err != nil {
		tools.ErrorLogger.Printf(msg.TakeDataFail, err.Error())
		return err
	}

	return nil
}

func AddNewSlave(newSlaveAddress string, targetMaster RedisClient) error {

	addClientMutex.Lock()
	defer addClientMutex.Unlock()

	if err := NodeConnectionSetup([]string{newSlaveAddress}, AddSlave); err != nil {
		return err
	}

	newSlave, err := GetSlaveClientWithAddress(newSlaveAddress)
	if err != nil {
		tools.ErrorLogger.Printf(msg.NewSlaveNotFound)
		return err
	}

	// 모니터 서버에도 등록 요청
	if _, err := monitorClient.ask(*newSlave, NewConnect); err != nil {
		tools.ErrorLogger.Printf(msg.MonitorRegisterFail)
		return err
	}

	if err := createDataLogFile(newSlaveAddress); err != nil {
		return err
	}

	initMasterSlaveMaps(targetMaster, *newSlave)

	// 기존 마스터의 데이터 복사
	if err := targetMaster.copyDataTo(*newSlave); err != nil {
		return err
	}

	return nil
}

func AppendMaster(masterClient RedisClient) {
	redisMasterClients = append(redisMasterClients, masterClient)
}

func AppendSlave(slaveClient RedisClient) {
	redisSlaveClients = append(redisSlaveClients, slaveClient)
}

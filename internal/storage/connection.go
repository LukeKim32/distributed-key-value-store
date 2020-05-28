package storage

import (
	"fmt"
	"hash_interface/internal/hash"
	msg "hash_interface/internal/storage/message"
	"hash_interface/tools"
	"sync"

	"github.com/gomodule/redigo/redis"
)

const (
	// ConnTimeoutDuration unit is nanoseconds
	ConnTimeoutDuration = 2000000000
)

type ConnectOption uint8

const (
	Default ConnectOption = iota
	InitSlaveSetup
	AddSlave
)

func init() {
	if masterSlaveMap == nil {
		masterSlaveMap = make(map[string]RedisClient)
	}
	if slaveMasterMap == nil {
		slaveMasterMap = make(map[string]RedisClient)
	}
	if MasterSlaveChannelMap == nil {
		MasterSlaveChannelMap = make(map[string](chan MasterSlaveMessage))
	}
	if redisMutexMap == nil {
		redisMutexMap = make(map[string]*sync.Mutex)
	}
	if clientHashRangeMap == nil {
		clientHashRangeMap = make(map[string][]HashRange)
	}
	if addClientMutex == nil {
		addClientMutex = &sync.Mutex{}
	}
}

func NodeConnectionSetup(addressList []string, connectOption ConnectOption) error {

	var err error

	for i, eachNodeAddress := range addressList {
		newRedisClient := RedisClient{
			Address: eachNodeAddress,
		}

		if newRedisClient.isAlreadyExist() {
			return fmt.Errorf(msg.ClientAlreadyExist)
		}

		newRedisClient.Connection, err = redis.Dial(
			"tcp",
			eachNodeAddress,
			redis.DialConnectTimeout(ConnTimeoutDuration),
		)
		if err != nil {
			tools.ErrorLogger.Printf(
				msg.ConnectionFailure,
				eachNodeAddress,
				err.Error(),
			)
			return err
		}

		switch connectOption {
		case Default:

			newRedisClient.Role = MasterRole
			redisMasterClients = append(
				redisMasterClients,
				newRedisClient,
			)

		case InitSlaveSetup:

			if len(redisMasterClients) == 0 {
				return fmt.Errorf(msg.RedisMasterNotSetUpYet)
			}
			if len(redisMasterClients) > len(addressList) {
				return fmt.Errorf(msg.SlaveNumberMustBeLarger)
			}

			// Modula index for circular assignment
			index := i % len(redisMasterClients)
			targetMasterClient := redisMasterClients[index]

			newRedisClient.Role = SlaveRole
			redisSlaveClients = append(redisSlaveClients, newRedisClient)

			// Mutex for each Master-Slave set
			redisMutexMap[targetMasterClient.Address] = &sync.Mutex{}
			initMasterSlaveMaps(targetMasterClient, newRedisClient)

			// tools.InfoLogger.Printf(
			// 	msg.SlaveMappedToMaster,
			// 	newRedisClient.Address,
			// 	targetMasterClient.Address,
			// )

		case AddSlave:

			if len(addressList) != 1 {
				return fmt.Errorf(msg.AddSlaveParameterEror)
			}

			newRedisClient.Role = SlaveRole
			redisSlaveClients = append(redisSlaveClients, newRedisClient)

		}

		//tools.InfoLogger.Printf(msg.NodeConnectSuccess, eachNodeAddress)
	}

	return nil
}

func MakeHashMapToRedis() error {

	connectionCount := len(redisMasterClients)
	if connectionCount == 0 {
		return fmt.Errorf(msg.RedisMasterNotSetUpYet)
	}

	for i, eachRedisNode := range redisMasterClients {
		// arithmatic order fixed to prevent Mantissa Loss
		hashSlotStart := uint16(
			float64(i) / float64(connectionCount) * float64(hash.HashSlotsNumber),
		)
		hashSlotEnd := uint16(
			float64(i+1) / float64(connectionCount) * float64(hash.HashSlotsNumber),
		)

		hashSlot.assign(&redisMasterClients[i], hashSlotStart, hashSlotEnd)

		newHashRange := HashRange{
			startIndex: hashSlotStart,
			endIndex:   hashSlotEnd,
		}
		clientHashRangeMap[eachRedisNode.Address] = append(
			clientHashRangeMap[eachRedisNode.Address],
			newHashRange,
		)

		// tools.InfoLogger.Printf(
		// 	msg.HashSlotAssignResult,
		// 	eachRedisNode.Address,
		// 	hashSlotStart,
		// 	hashSlotEnd-1,
		// )
	}

	return nil
}

func initMasterSlaveMaps(masterNode RedisClient, slaveNode RedisClient) {

	masterSlaveMap[masterNode.Address] = slaveNode
	slaveMasterMap[slaveNode.Address] = masterNode

	redisMutexMap[slaveNode.Address] = redisMutexMap[masterNode.Address]
}

func (slaveClient *RedisClient) connectToMaster(masterClient *RedisClient) error {

	// tools.InfoLogger.Printf(
	// 	msg.ConnectSlaveToMaster,
	// 	slaveClient.Address,
	// 	masterClient.Address,
	// )

	err := createDataLogFile(slaveClient.Address)
	if err != nil {
		return err
	}

	if err := slaveClient.TryReconnect(); err != nil {
		tools.ErrorLogger.Printf(
			msg.ConnectionFailure,
			slaveClient.Address,
			err.Error(),
		)
		return err
	}

	// 슬레이브 환경 설정
	slaveClient.Role = SlaveRole
	redisSlaveClients = append(redisSlaveClients, *slaveClient)
	initMasterSlaveMaps(*masterClient, *slaveClient)

	// 기존 마스터의 데이터 복사
	if err := masterClient.copyDataTo(*slaveClient); err != nil {
		tools.ErrorLogger.Printf(
			"슬레이브(%s)에 데이터 복사 에러 : %s",
			slaveClient.Address,
			err.Error(),
		)
		return err
	}

	// 모니터링 하는 레디스에서 제거
	if _, err := monitorClient.ask(*slaveClient, EndConnect); err != nil {
		tools.ErrorLogger.Printf("모니터 서버에 %s 제거 요청 실패!", slaveClient.Address)
		return err
	}

	tools.ErrorLogger.Printf("모니터 서버에 %s 제거 요청 성공!", slaveClient.Address)

	// 모니터 서버에게 연결 확인 요청
	if _, err := monitorClient.ask(*slaveClient, NewConnect); err != nil {
		tools.ErrorLogger.Printf("모니터 서버에 %s 등록 요청 실패!", slaveClient.Address)
		return err
	}

	tools.ErrorLogger.Printf("모니터 서버에 %s 등록 요청 성공!", slaveClient.Address)

	// tools.InfoLogger.Printf(
	// 	msg.SlaveMappedToMaster,
	// 	slaveClient.Address,
	// 	masterClient.Address,
	// )
	// tools.InfoLogger.Printf(
	// 	msg.NodeConnectSuccess,
	// 	slaveClient.Address,
	// )

	return nil
}

func (redisClient *RedisClient) TryReconnect() error {
	var err error

	redisClient.Connection, err = redis.Dial(
		"tcp",
		redisClient.Address,
		redis.DialConnectTimeout(ConnTimeoutDuration),
	)
	if err != nil {
		tools.ErrorLogger.Printf(msg.ReconnectFail, redisClient.Address)
		return err

	}

	tools.ErrorLogger.Printf(msg.ReconnectSuccess, redisClient.Address)
	return nil
}

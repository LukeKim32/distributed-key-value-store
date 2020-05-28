package storage

import (
	"fmt"

	"github.com/gomodule/redigo/redis"

	msg "hash_interface/internal/storage/message"
	"hash_interface/tools"
)

/************* Docker를 계속 restart 해준다는 가정 ******************/

// migrateDataToOthers : deadClient 인스턴스의 데이터를 다른 마스터-슬레이브 세트에 할당
//  - 과정 :
//  1. deadClient의 데이터 로그 파일 읽기 => 최신 데이터 현황 생성
//  2. deadClient를 제외한 다른 마스터에 데이터 분배
//
func (deadClient RedisClient) migrateDataToOthers() error {

	// deadClient의 로그 파일 읽기 => 최신 데이터 현황 생성
	deadClientDataContainer := make(HashToDataMap)
	err := deadClient.getLatestDataFromLog(deadClientDataContainer)
	if err != nil {
		return err
	}

	// deadClient (마스터) 인스턴스의 데이터 로그 파일 삭제
	if err := deadClient.removeDataLogFile(); err != nil {
		return err
	}

	// deadClient의 슬레이브의 로그 파일이 존재한다면 삭제
	deadSlave, isExist := masterSlaveMap[deadClient.Address]
	if isExist {
		deadSlave.removeDataLogFile()
	}

	// deadClient의 데이터를, 해쉬 슬롯에 새로 매핑된 다른 마스터에 할당
	for hashIndex, keyValueMap := range deadClientDataContainer {

		newMappedClient := hashSlot.slots[hashIndex]

		for eachKey, eachValue := range keyValueMap {

			// tools.InfoLogger.Printf(
			// 	msg.MigrateDataFromTo,
			// 	deadClient.Address,
			// 	eachKey,
			// 	eachValue,
			// 	newMappedClient.Address,
			// )

			// 레디스에 저장
			_, err := redis.String(newMappedClient.Connection.Do("SET", eachKey, eachValue))
			if err != nil {
				return err
			}

			// 저장 목표 마스터가 중간에 죽어도, 로그 파일에는 기록을 남김
			err = newMappedClient.RecordModificationLog("SET", eachKey, eachValue)
			if err != nil {
				return fmt.Errorf(msg.LogFailWhileMigration, deadClient.Address)
			}

			// 저장 목표 마스터의 슬레이브에게도 전파
			newMappedClient.ReplicateToSlave("SET", eachKey, eachValue)
		}
	}

	return nil

}

// reshardData : 모든 마스터 클라이언트의 최신 데이터 로그 생성 & 현재 해쉬슬롯 기준 데이터 재분배
//
func (redisClient RedisClient) reshardData() error {

	for _, srcMasterClient := range redisMasterClients {

		// 마스터 클라이언트의 로그 파일 읽기 => 최신 데이터 현황 생성
		dataOfSrcMaster := make(HashToDataMap)

		err := srcMasterClient.getLatestDataFromLog(dataOfSrcMaster)
		if err != nil {
			return err
		}

		// 데이터 분배 후 데이터 최신 현황 변경에 따른, 로그 파일 재생성
		srcMasterClient.removeDataLogFile()

		if err := srcMasterClient.createDataLogFile(); err != nil {
			return err
		}

		// 마스터 클라이언트의 데이터를, 갱신된 해쉬 슬롯에 매핑된 마스터들에게 할당
		for hashIndex, keyValueMap := range dataOfSrcMaster {

			newMappedClient := hashSlot.slots[hashIndex]

			// 갱신된 해쉬 슬롯에 매핑된 마스터가 변하지 않은 경우
			if newMappedClient.Address == srcMasterClient.Address {

				for eachKey, eachValue := range keyValueMap {

					err := newMappedClient.RecordModificationLog("SET", eachKey, eachValue)
					if err != nil {
						return fmt.Errorf(msg.LogFailWhileMigration, newMappedClient.Address)
					}
				}

			} else {
				// 새로 매핑된 마스터인 경우

				for eachKey, eachValue := range keyValueMap {

					tools.ErrorLogger.Printf("데이터 로그 key : %s, value : %s", eachKey, eachValue)

					redisResponse, err := redis.String(srcMasterClient.Connection.Do("GET", eachKey))
					if err == redis.ErrNil {
						redisResponse = "nil(없음)"

					} else if err != nil {
						tools.ErrorLogger.Printf("데이터 가져오기 실패 key : %s, Error : %s", eachKey, err.Error())
					}

					tools.InfoLogger.Printf("키 : %s, 값 : %s", eachKey, redisResponse)

					// 기존 데이터 주인이었던 마스터 클라이언트에서는 제거
					_, err = srcMasterClient.Connection.Do("DEL", eachKey)
					if err != nil {
						tools.ErrorLogger.Printf("데이터 삭제간 에러!")
						return fmt.Errorf(msg.DeleteDataFail, err.Error())
					}

					// tools.InfoLogger.Printf(
					// 	msg.MigrateDataFromTo,
					// 	srcMasterClient.Address,
					// 	eachKey,
					// 	eachValue,
					// 	newMappedClient.Address,
					// )

					// 새로 매핑된 마스터에 저장
					_, err = redis.String(newMappedClient.Connection.Do("SET", eachKey, eachValue))

					// 새로 매핑된 마스터가 중간에 죽어도, 로그 파일에는 기록을 해놓는다
					err = newMappedClient.RecordModificationLog("SET", eachKey, eachValue)
					if err != nil {
						return fmt.Errorf(msg.LogFailWhileMigration, newMappedClient.Address)
					}

					// 데이터를 redisClient로 옮긴 후, redisClient의 슬레이브에게도 전파
					newMappedClient.ReplicateToSlave("SET", eachKey, eachValue)
				}
			}
		}
	}

	return nil

}

// copyDataTo : masterClient의 데이터를 슬레이브에 복사
//  데이터 로그파일을 읽어 최신 데이터 만을 복사한다
//
func (masterClient RedisClient) copyDataTo(slaveClient RedisClient) error {

	// masterClient의 최신 데이터 현황 생성
	masterDataContainer := make(HashToDataMap)
	if err := masterClient.getLatestDataFromLog(masterDataContainer); err != nil {
		return err
	}

	for _, keyValueMap := range masterDataContainer {

		for eachKey, eachValue := range keyValueMap {

			// tools.InfoLogger.Printf(
			// 	msg.CopyFromMasterToSlave,
			// 	masterClient.Address,
			// 	eachKey,
			// 	eachValue,
			// 	slaveClient.Address,
			// )

			// 슬레이브에 데이터 복사
			_, err := redis.String(slaveClient.Connection.Do("SET", eachKey, eachValue))
			if err != nil {
				return err
			}

			// 슬레이브가 중간에 죽어도, 로그 파일에는 기록을 해놓는다
			err = slaveClient.RecordModificationLog("SET", eachKey, eachValue)
			if err != nil {
				tools.ErrorLogger.Printf(msg.LogFailWhileMigration, slaveClient.Address)
			}
		}
	}

	return nil
}

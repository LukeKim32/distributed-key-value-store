package storage

import (
	"fmt"
	msg "hash_interface/internal/storage/message"
	"hash_interface/tools"
	"sync"
)

type HashSlot struct {
	slots             map[uint16]*RedisClient
	redistributeMutex *sync.Mutex
}

var hashSlot HashSlot

// hashSlot is a map of (Hash Slot -> Redis Node Client)
// var hashSlot map[uint16]*RedisClient

// redistributeSlotMutex : 해쉬 슬롯 재분배 시 전체 Client Lock
// used for sync in accessing Hash Maps After Redistribution
// var redistributeSlotMutex = &sync.Mutex{}

// clientHashRangeMap : Redis Client 주소 -> 담당하는 해쉬 슬롯 구간들
var clientHashRangeMap map[string][]HashRange

type HashRange struct {
	startIndex uint16
	endIndex   uint16
}

func init() {
	if hashSlot.slots == nil {
		hashSlot.slots = make(map[uint16]*RedisClient)
	}
	if hashSlot.redistributeMutex == nil {
		hashSlot.redistributeMutex = &sync.Mutex{}
	}
}

func (hashSlot HashSlot) get(slotIndex uint16) RedisClient {
	return *hashSlot.slots[slotIndex]
}

// assign : 인스턴스에 [@start, @end) 범위에 해당하는 해쉬 슬롯 할당
// Loop unrolling을 이용, 퍼포먼스 최적화
// assigns Hash slots (@start ~ @end) to passed @redisClient
// It basically unrolls the loop with 16 states for cahching
// And If the range Is not divided by 16, Remains will be handled with single statement loop
//
func (hashSlot HashSlot) assign(redisClient *RedisClient, start uint16, end uint16) {

	// tools.InfoLogger.Printf(msg.HashSlotAssignStart, redisClient.Address)

	var i uint16
	nextSlotIndex := start + 16
	// Replace Hash Map With Slave Client
	for i = start; nextSlotIndex < end; i += 16 {
		hashSlot.slots[i] = redisClient
		hashSlot.slots[i+1] = redisClient
		hashSlot.slots[i+2] = redisClient
		hashSlot.slots[i+3] = redisClient
		hashSlot.slots[i+4] = redisClient
		hashSlot.slots[i+5] = redisClient
		hashSlot.slots[i+6] = redisClient
		hashSlot.slots[i+7] = redisClient
		hashSlot.slots[i+8] = redisClient
		hashSlot.slots[i+9] = redisClient
		hashSlot.slots[i+10] = redisClient
		hashSlot.slots[i+11] = redisClient
		hashSlot.slots[i+12] = redisClient
		hashSlot.slots[i+13] = redisClient
		hashSlot.slots[i+14] = redisClient
		hashSlot.slots[i+15] = redisClient
		nextSlotIndex += 16
	}

	for ; i < end; i++ {
		hashSlot.slots[i] = redisClient
	}

	// tools.InfoLogger.Printf(msg.HashSlotAssignFinish, redisClient.Address)

}

// distributeFrom : srcClient 인스턴스에게 할당된 해쉬 슬롯을 다른 Redis 마스터 Client 들에게 분배.
//  완료 후, srcClient는 해쉬슬롯에서 제거된다.
//
func (hashSlot HashSlot) distributeFrom(srcClient *RedisClient) error {

	// tools.InfoLogger.Printf(msg.HashSlotRedistributeStart, srcClient.Address)
	// tools.InfoLogger.Printf(msg.DeadRedisNodeInfo, srcClient.Address, srcClient.Role)

	if len(clientHashRangeMap[srcClient.Address]) == 0 {
		return fmt.Errorf(msg.NoHashRangeIsAssigned, srcClient.Address)
	}

	hashSlot.redistributeMutex.Lock()
	defer hashSlot.redistributeMutex.Unlock()

	restOfMasterNumber := len(redisMasterClients) - 1

	if restOfMasterNumber < 1 {
		// tools.ErrorLogger.Println(msg.NoMasterClients)

		// To-Do 모두 초기화 처리

		return fmt.Errorf(msg.NoMasterClients)
	}

	// srcClient가 담당하던 해쉬 슬롯 범위에 대해
	for _, eachHashRangeOfClient := range clientHashRangeMap[srcClient.Address] {

		srcHashSlotStart := eachHashRangeOfClient.startIndex
		srcHashSlotEnd := eachHashRangeOfClient.endIndex
		srcHashSlotRange := srcHashSlotEnd - srcHashSlotStart + 1

		i := 0 // 임의의 마스터 클라이언트 인덱스
		// 다른 마스터에게 해쉬 슬롯 균일 분배
		for idx, eachMasterNode := range redisMasterClients {

			if eachMasterNode.Address != srcClient.Address {

				// 소수부 손실을 막기 위한 계산 순서
				// arithmatic order fixed to prevent Mantissa Loss
				normalizedHashSlotStart := uint16(
					float64(i) / float64(restOfMasterNumber) * float64(srcHashSlotRange),
				)
				normalizedhashSlotEnd := uint16(
					float64(i+1) / float64(restOfMasterNumber) * float64(srcHashSlotRange),
				)

				hashSlotStart := normalizedHashSlotStart + srcHashSlotStart
				hashSlotEnd := normalizedhashSlotEnd + srcHashSlotStart
				hashSlot.assign(&redisMasterClients[idx], hashSlotStart, hashSlotEnd)

				newHashRange := HashRange{
					startIndex: hashSlotStart,
					endIndex:   hashSlotEnd,
				}

				clientHashRangeMap[eachMasterNode.Address] = append(
					clientHashRangeMap[eachMasterNode.Address],
					newHashRange,
				)
				i++
			}
		}
	}

	// srcClient가 저장하고 있던 데이터 Migration
	if err := srcClient.migrateDataToOthers(); err != nil {
		return err
	}

	if err := srcClient.cleanUpMemory(); err != nil {
		return err
	}

	// Print Current Updated Masters
	// PrintCurrentMasterSlaves()

	//tools.InfoLogger.Printf(msg.HashSlotRedistributeFinish, srcClient.Address)

	return nil
}

func PrintCurrentMasterSlaves() {

	for _, eachMaster := range redisMasterClients {
		tools.InfoLogger.Printf(msg.RefreshedMasters, eachMaster.Address)
		tools.InfoLogger.Printf(
			msg.RedisRole,
			eachMaster.Address,
			eachMaster.Role,
		)
	}

	for _, eachSlave := range redisSlaveClients {
		tools.InfoLogger.Printf(msg.RefreshedSlaves, eachSlave.Address)
		tools.InfoLogger.Printf(
			msg.RedisRole,
			eachSlave.Address,
			eachSlave.Role,
		)
	}
}

// distributeTo :  새로운 마스터에게 할당 될 해쉬 슬롯은
// "각 마스터의 해쉬슬롯 / 총 마스터 클라이언트 수" 크기만큼
// 기존 마스터들로부터 재분배
// * @destClient 이외의 마스터에겐 해쉬슬롯이 분배되지 않는다.
//
func (hashSlot HashSlot) distributeTo(destClient *RedisClient) bool {

	notDistributed := false

	for _, eachMaster := range redisMasterClients {
		//fmt.Printf("distributeTo() : 소스 마스터 노드 주소 - %s\n", eachMaster.Address)
		// 새로 추가된 마스터가 아닌 경우
		if eachMaster.Address != destClient.Address {

			// fmt.Printf(
			// 	"distributeTo() : 소스 마스터 할당 해쉬 슬롯 범위 개수 : %d\n",
			// 	len(clientHashRangeMap[eachMaster.Address]),
			// )

			// for _, eachRange := range clientHashRangeMap[eachMaster.Address] {

			// fmt.Printf(
			// 	"distributeTo() : 소스 마스터 할당 해쉬 슬롯 범위 (%d ~ %d)\n",
			// 	eachRange.startIndex,
			// 	eachRange.endIndex,
			// )
			// }

			// 기존 마스터가 담당하는 해쉬 슬롯 범위들
			for idx, eachRange := range clientHashRangeMap[eachMaster.Address] {

				srcHashSlotRange := eachRange.endIndex - eachRange.startIndex + 1

				// 새로운 마스터가 할당 받은 해쉬슬롯 크기 : (기존 마스터 담당 해쉬 슬롯 / n)
				destHashSlotRange := uint16(
					float64(1) / float64(len(redisMasterClients)) * float64(srcHashSlotRange),
				)
				destHashSlotStart := eachRange.startIndex
				destHashSlotEnd := destHashSlotStart + destHashSlotRange

				// To-Do :해쉬 슬롯을 더 이상 나눌 수 없을 정도로 레디스 클라이언트의 개수가 많아 질 때 에러

				hashSlot.assign(destClient, destHashSlotStart, destHashSlotEnd)

				newHashRange := HashRange{
					startIndex: destHashSlotStart,
					endIndex:   destHashSlotEnd,
				}

				clientHashRangeMap[destClient.Address] = append(
					clientHashRangeMap[destClient.Address],
					newHashRange,
				)

				// 기존 마스터가 나눠준 해쉬 슬롯 범위에 맞게 수정
				clientHashRangeMap[eachMaster.Address][idx].startIndex = destHashSlotEnd

				tools.InfoLogger.Printf(
					msg.HashSlotAssignResult,
					destClient.Address,
					destHashSlotStart,
					destHashSlotEnd-1,
				)

				// 분배된 적 있는지 체크하는 프래그
				if !notDistributed {
					notDistributed = true
				}
			}
		}
	}

	return notDistributed
}

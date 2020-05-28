package storage

import (
	"hash_interface/internal/storage/message"
	"hash_interface/tools"
	"time"
)

// @Deprecated
// MasterSlaveChannelMap : 마스터 IP 주소 -> 슬레이브 Client, 마스터-슬레이브간 메세지 교환 채널
var MasterSlaveChannelMap map[string](chan MasterSlaveMessage)

// @Deprecated
// MasterSlaveMessage : 마스터-슬레이브간 메세지 포맷
type MasterSlaveMessage struct {
	MasterNode RedisClient
	SlaveNode  RedisClient
	// isCurrentSlaveDead is the status of current Struct Variable's "SlaveNode" field
	isCurrentSlaveDead bool
}

// StartMonitorNodes : 매 초 Redis Client들의 상태 확인/처리
func StartMonitorNodes() {
	errorChannel = make(chan error)
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:

				if err := checkRedisClientSetup(); err != nil {
					errorChannel <- err
				}

				for _, eachMasterClient := range redisMasterClients {
					go eachMasterClient.handleIfDeadWithLock(errorChannel)
				}

			case <-errorChannel:
				ticker.Stop()
				close(errorChannel)
				tools.ErrorLogger.Println(message.MonitorNodesError)
				return
			}
		}
	}()
}

package storage

import (
	"github.com/gomodule/redigo/redis"
)

// ReplicateToSlave : masterClient 인스턴스의 슬레이브에게 명령 전파
// 슬레이브가 죽어있으면 처리하지 않는다 (살아날 때 마스터의 데이터를 복사)
//
func (masterClient RedisClient) ReplicateToSlave(command string, key string, value string) {

	//tools.InfoLogger.Println(msg.StartReplicaiton)

	// 슬레이브가 죽은 경우, 에러
	slaveClient, err := masterClient.getSlave()
	if err != nil {
		return
	}

	// 슬레이브가 살아있는 경우
	redis.String(slaveClient.Connection.Do(command, key, value))

	slaveClient.RecordModificationLog(command, key, value)

	//tools.InfoLogger.Println(msg.EndReplication)
}

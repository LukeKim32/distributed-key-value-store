package configs

import (
	"os"
)

const (
	// HTTP protocol
	HTTP = "http://"
	// HTTPS protocol
	HTTPS = "https://"
	// BaseURL consists of IP address + Port + specific path
	BaseURL = "localhost/interface"
	// Port is 8080
	Port = 8888
	// JSONContent is for response header
	JsonContent = "application/json"
	// CORSheader is a header field for Cross Origin Resource Sharing Problem Solve
	CORSheader     = "Access-Control-Allow-Origin"
	ContentType    = "Content-Type"
	BadRequestBody = "Unappropriate Request Body"

	ApiDocumentPath = HTTP + BaseURL + "/docs/index.html"

	// Redis Master Node #1 (Container name : redis_one)
	RedisMasterOneAddress = "172.29.0.4:8000"
	// Redis Master Node #2 (Container name : redis_two)
	RedisMasterTwoAddress = "172.29.0.5:8001"
	// Redis Master Node #3 (Container name : redis_three)
	RedisMasterThreeAddress = "172.29.0.6:8002"
	// Redis Slave Node #1 (Container name : redis_four)
	RedisSlaveOneAddress = "172.29.0.7:8000"
	// Redis Master Node #5 (Container name : redis_five)
	RedisSlaveTwoAddress = "172.29.0.8:8001"
	// Redis Master Node #6 (Container name : redis_six)
	RedisSlaveThreeAddress = "172.29.0.9:8002"

	InterfaceNodeAddress  = "172.29.0.3:8888"
	MonitorNodeOneAddress = "172.29.0.10:8888"
	MonitorNodeTwoAddress = "172.29.0.11:8888"
)

// CurrentIP is IP address of Go-application, will be initialized in main.go
var CurrentIP = os.Getenv("DOCKER_HOST_IP")

func GetInitialMasterAddressList() []string {
	return []string{
		RedisMasterOneAddress,
		RedisMasterTwoAddress,
		RedisMasterThreeAddress,
	}
}

func GetInitialSlaveAddressList() []string {
	return []string{
		RedisSlaveOneAddress,
		RedisSlaveTwoAddress,
		RedisSlaveThreeAddress,
	}
}

func GetInitialTotalAddressList() []string {
	return []string{
		RedisMasterOneAddress,
		RedisMasterTwoAddress,
		RedisMasterThreeAddress,
		RedisSlaveOneAddress,
		RedisSlaveTwoAddress,
		RedisSlaveThreeAddress,
	}
}

var ServerIpToDomainMap map[string]string

const (
	intern001 = "10.106.163.150"
	raft001   = "10.113.92.25"
	raft002   = "10.113.95.248"
	raft003   = "10.113.219.74"
	raft004   = "10.113.101.1"
)

func init() {
	if ServerIpToDomainMap == nil {
		ServerIpToDomainMap = make(map[string]string)
		ServerIpToDomainMap[intern001] = "RAFT_A"
		ServerIpToDomainMap[raft001] = "RAFT_B"
		ServerIpToDomainMap[raft002] = "RAFT_C"
		ServerIpToDomainMap[raft003] = "RAFT_D"
		ServerIpToDomainMap[raft004] = "RAFT_E"
	}
}

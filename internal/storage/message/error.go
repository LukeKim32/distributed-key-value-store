package message

const (
	/* Error Log Templates */
	MasterSlaveMapNotInit    string = "Master Slave Map is not set up"
	BothMasterSlaveDead             = "마스터와 슬레이브 모두 죽었음"
	ConnectionFailure               = "레디스 클라이언트(%s) 연결 실패 - %s"
	RedisMasterNotSetUpYet          = "Redis Master Node should be set up first"
	RedisSlaveNotSetUpYet           = "Redis Slave Node should be set up first"
	NotAnyRedisSetUpYet             = "GetRedisClientWithAddress() error : No Redis Clients set up"
	SlaveNumberMustBeLarger         = "The number of Slave Nodes should be bigger than Master's"
	NoMatchingSlaveToRemove         = "RemoveSlaveFromList() : No Matching Redis Client to Remove - %s"
	NoMatchingMasterToRemove        = "RemoveMasterFromList() : No Matching Redis Client to Remove - %s"
	NoMatchingRedisToFind           = "GetRedisClientWithAddress() error : No Matching Redis Client With passed address"
	NoHashRangeIsAssigned           = "distributeFrom() : No Hash Range is assigned to Node(%s)"
	MonitorNodesError               = "MonitorMasters() : Error - timer stopped"
	ResponseMonitorError            = "requestToMonitor() :Monitor server(IP : %s) response error : %s"
	NoMatchingResponseNode          = "Reuqested Redis Node Address Not Match with Response"
	NotAllowedIfNotMaster           = "handleIfDead() Error : Method is only allowed to Master"
	SlaveNotFound                   = "GetSlaveClientWithAddress() : 타겟 못찾음"
	NewMasterNotFound               = "마스터 찾지 못함"
	MonitorRegisterFail             = "모니터 서버에 신규 마스터 등록 실패"
	DistributeToFail                = "클라이언트(%s) <= 해쉬슬롯 재분배 실패"
	TakeDataFail                    = "데이터 가져오기 실패 - %s"
	NewSlaveNotFound                = "새로 추가된 슬레이브 찾지 못함"
	ClientAlreadyExist              = "레디스 클라이언트(%s)는 이미 등록되어 있습니다."
	AddSlaveParameterEror           = "NodeConnectionSetup() : AddSlave option needs 1 address"
	ReconnectFail                   = "Redis Node(%s) 재연결 시도 실패"
	NoMasterClients                 = "살아있는 Master Node가 없습니다."
	DeleteDataFail                  = "reshardDataTo() : deleting from source node error - %s"
	VoteResultSlaveDead             = "투표 결과 : 슬레이브(%s) 죽음"
	ConnectionCloseFailure          = "RemoveFromList() : 레디스(%s) 커넥션 닫기 에러"
	RedisRoleNotInit                = "RemoveFromList() : Redis Client(%s) Role has not been set!"
	NoClientInList                  = "RemoveFromList() : Client(%s) not in its %s list"

	/* Data Log Related Messages*/
	CreateLogFileError        = "데이터 로그파일 생성 오류"
	DataLoggerSetupError      = "data Logger is not set up"
	DataLogOpenError          = "데이터 로그파일 %s 열기 에러 - %s"
	ParseHashIndexStringError = "데이터 로그의 해쉬 인덱스 파싱 에러"
	UnsupportedCommand        = "데이터 로그에서 지원하지 않는 명령(%s) 읽음"
	FileScannerError          = "데이터 로그 스캐너 에러 - %s"
	RemoveLogFileError        = "데이터 로그 파일 %s 삭제 에러 - %s"
	LogFailWhileMigration     = "노드(%s)의 데이터 로그 기록 중 에러"

	/* Monitor server Messages */
	UnsupportedMonitorRequest = "Moniter Client ask() : 지원하지 않는 옵션"
	MonitorRequestTimeout     = "모니터 서버(%s) 요청 타임아웃(3sec) 에러"
	CreateRequestError        = "requestUnregister() : 요청 생성 에러 %s"

	DockerInitFail    = "docker client init error"
	ContainerNotFound = "No Such Container with IP : %s"
)

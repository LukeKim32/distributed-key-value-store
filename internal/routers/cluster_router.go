package routers

import (
	"net/http"

	"github.com/gorilla/mux"

	"hash_interface/internal/handlers"
)

func SetUpClusterRouter(router *mux.Router) {

	// Leader -> Followers
	// Write/Update Data를 Follower들의 Wal에 더함
	// 과반수 이상 OK 응답 => Commit Index 설정
	// 과반수 이하 OK 응답 => Commit X
	//
	router.HandleFunc("/wal/leader", handlers.HandleAppendWal).Methods(http.MethodPost)

	// Follower -> Leader
	// Write/Update Data 요청을 리더에게 전달
	//
	router.HandleFunc("/wal", handlers.HandleAppendWal).Methods(http.MethodPost)

	// Follower -> Leader
	// Wal 업데이트 요청
	//
	router.HandleFunc(
		"/update",
		handlers.StartUpdateFollowerWal,
	).Methods(http.MethodGet)

	// Leader -> Follower
	// Wal 업데이트 요청을 받아 데이터 전송
	//
	router.HandleFunc(
		"/update",
		handlers.UpdateWalFromLeader,
	).Methods(http.MethodPost)

	// Leader -> Follower
	// Wal 업데이트 요청을 받아 데이터 전송
	//
	router.HandleFunc(
		"/commit/{index}",
		handlers.HandleCommit,
	).Methods(http.MethodGet)

	// Leader -> Followers
	// Heartbeat
	//
	router.HandleFunc("/heartbeat", handlers.HandleHeartbeat).Methods(http.MethodGet)

	router.HandleFunc("/election", handlers.Vote).Methods(http.MethodGet)

	// Client API
	router.HandleFunc("/leader", handlers.PrintLeader).Methods(http.MethodGet)

	// Client API
	router.HandleFunc("", handlers.RegisterNode).Methods(http.MethodPost)

	// Client API
	router.HandleFunc("", handlers.StartCluster).Methods(http.MethodPut)

	// Client API
	router.HandleFunc("", handlers.PrintRegisteredNodes).Methods(http.MethodGet)

}

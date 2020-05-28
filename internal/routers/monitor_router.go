package routers

import (
	"github.com/gorilla/mux"
	"hash_interface/internal/handlers"
	"net/http"
)

func SetUpMonitorRouter(router *mux.Router) {

	// 새로 모니터할 레디스 클라이언트 등록
	router.PathPrefix("/connect/{redis_address}").HandlerFunc(handlers.RegisterNewRedis).Methods(http.MethodPost)

	// 모니터링 중인 레디스 클라이언트 삭제
	router.PathPrefix("/connect/{redis_address}").HandlerFunc(handlers.UnregisterRedis).Methods(http.MethodDelete)

	// 모니터링 중인 레디스 클라이언트 Alive 테스트
	router.HandleFunc("/{redis_address}", handlers.CheckRedisNodeStatus).Methods(http.MethodGet)

	// 현재 등록된 모니터링 중인 레디스 클라이언트 출력
	router.HandleFunc("/nodes", handlers.ShowCurrentRedisList).Methods(http.MethodGet)
}

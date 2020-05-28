package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"hash_interface/configs"
	"hash_interface/internal/handlers"
	"hash_interface/internal/routers"
	"hash_interface/internal/storage"
	"hash_interface/tools"

	_ "hash_interface/docs"

	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title Redis-Cluster/Sentinel-Like Server
// @version 1.1
// @description This service presents group of Redis nodes and the interface of them
// @description to test master/slave replication, data persistence, failover redemption

// @contact.name 김예찬 (Luke Kim)
// @contact.email burgund32@gmail.com

// @host localhost:8888
// @BasePath /api/v1
func main() {

	fmt.Println("호스트 IP : ", os.Getenv("DOCKER_HOST_IP"))

	tools.SetUpLogger("hash_server")

	// Redis Master Containers들과 Connection설정
	err := storage.NodeConnectionSetup(
		configs.GetInitialMasterAddressList(),
		storage.Default,
	)

	if err != nil {
		tools.ErrorLogger.Fatalln(
			"Error - Node connection error : ",
			err.Error(),
		)
	}

	// create Hash Map (Index -> Redis Master Nodes)
	if err := storage.MakeHashMapToRedis(); err != nil {
		tools.ErrorLogger.Fatalln(
			"Error - Redis Node Address Mapping to Hash Map failure: ",
			err.Error(),
		)
	}

	// Redis Slave Containers들과 Connection설정
	err = storage.NodeConnectionSetup(
		configs.GetInitialSlaveAddressList(),
		storage.InitSlaveSetup,
	)
	if err != nil {
		tools.ErrorLogger.Fatalln(
			"Error - Node connection error : ",
			err.Error(),
		)
	}

	// storage.PrintCurrentMasterSlaves()

	/* Set Data modification Logger for each Nodes*/
	storage.SetUpModificationLogger(configs.GetInitialTotalAddressList())

	// 타이머로 Redis Node들 모니터링 시작
	// storage.StartMonitorNodes()

	router := mux.NewRouter()

	router.PathPrefix("/api/v1/docs/").
		Handler(httpSwagger.WrapHandler)

	// Interface Server Router 설정
	routers.SetUpClusterRouter(
		router.PathPrefix("/api/v1/cluster").
			Subrouter(),
	)
	routers.SetUpInterfaceRouter(
		router.PathPrefix("/api/v1").
			Subrouter(),
	)

	// 허용하지 않는 URL 경로 처리
	router.PathPrefix("/").HandlerFunc(handlers.ExceptionHandle)
	http.Handle("/", router)

	// Raft Setup

	tools.InfoLogger.Println("Server start listening on port ", configs.Port)

	tools.ErrorLogger.Fatal(
		http.ListenAndServe(":"+strconv.Itoa(configs.Port), router),
	)
}

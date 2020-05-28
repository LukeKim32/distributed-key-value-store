package main

import (
	"net/http"
	"strconv"

	"hash_interface/configs"
	"hash_interface/internal/handlers"
	"hash_interface/internal/routers"
	"hash_interface/internal/storage"
	"hash_interface/tools"

	"github.com/gorilla/mux"
)

/*
 * 모니터 서버는 마스터와 슬레이브를 구분하지 않는다.
 */

// @title Redis Cluster Interface Test Server
// @version 1.0

// @contact.name 김예찬
// @contact.email burgund32@gmail.com
// @BasePath /api/v1/projects
func main() {

	var err error

	tools.SetUpLogger("monitor_server")

	// To Check Load Balancing By Proxy
	if configs.CurrentIP, err = tools.GetCurrentServerIP(); err != nil {
		tools.ErrorLogger.Fatalln(
			"Error - Get Go-App IP error : ", err.Error())
	}

	// Redis Master Containers들과 Connection설정
	err = storage.NodeConnectionSetup(
		configs.GetInitialMasterAddressList(),
		storage.Default,
	)
	if err != nil {
		tools.ErrorLogger.Fatalln("Error - Node connection error : ", err.Error())
	}

	// Redis Slave Containers들과 Connection설정
	err = storage.NodeConnectionSetup(
		configs.GetInitialSlaveAddressList(),
		storage.Default)
	if err != nil {
		tools.ErrorLogger.Fatalln("Error - Node connection error : ", err.Error())
	}

	router := mux.NewRouter()

	moniterRouter := router.PathPrefix("/monitor").Subrouter()

	// Monitor Server Router 설정
	routers.SetUpMonitorRouter(moniterRouter)

	// 허용하지 않는 URL 경로 처리
	router.PathPrefix("/").HandlerFunc(handlers.ExceptionHandle)
	http.Handle("/", router)

	tools.InfoLogger.Println("Server start listening on port ", configs.Port)

	tools.ErrorLogger.Fatal(
		http.ListenAndServe(":"+strconv.Itoa(configs.Port), router),
	)
}

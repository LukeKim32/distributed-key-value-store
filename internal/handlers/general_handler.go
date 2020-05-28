package handlers

import (
	"fmt"
	"hash_interface/configs"
	"hash_interface/internal/cluster"
	"hash_interface/internal/models/response"
	"hash_interface/tools"
	"net/http"
)

// ExceptionHandle handles request of unproper URL
func ExceptionHandle(res http.ResponseWriter, req *http.Request) {

	err := fmt.Errorf("Not a proper path usage")

	responseError(res, http.StatusTemporaryRedirect, err)

	tools.InfoLogger.Printf(
		"Not a proper path : %s\n",
		req.URL.String(),
	)
}

func responseError(res http.ResponseWriter, ErrorCode int, err error) {

	responseTemplate := response.BasicTemplate{}
	nextTaskMsg := fmt.Sprintf("Check the API document for proper use")

	responseBody, err := responseTemplate.Marshal(
		err.Error(),
		nextTaskMsg,
		configs.ApiDocumentPath,
	)
	if err != nil {
		tools.ErrorLogger.Println(err.Error())
		return
	}

	stateNode := cluster.StateNode

	indexTime := stateNode.GetIndexTime(true)
	indexTimeString := fmt.Sprintf(
		"%d",
		indexTime,
	)

	res.Header().Set(
		cluster.IndexTimeHeader,
		indexTimeString,
	)

	res.Header().Set(
		configs.ContentType,
		configs.JsonContent,
	)

	res.WriteHeader(ErrorCode)
	fmt.Fprint(res, string(responseBody))
}

func responseOK(res http.ResponseWriter, responseBody []byte) {

	// tools.InfoLogger.Println("Response back to client Successful")

	res.Header().Set(configs.ContentType, configs.JsonContent)
	res.WriteHeader(http.StatusOK)
	fmt.Fprint(res, string(responseBody))
}

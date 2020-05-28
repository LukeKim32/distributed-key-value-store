package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash_interface/configs"
	"hash_interface/internal/cluster"
	"hash_interface/internal/storage"
	"net/http"

	"hash_interface/internal/models"
	"hash_interface/internal/models/response"
)

// Naver LABS internal Server "http://10.113.93.194:8001"

func requestAddClientToServer(dataFlags ClientFlag) error {

	requestURI := fmt.Sprintf("%s/clients", baseUrl)

	client := &http.Client{}

	requestData := models.NewClientRequestContainer{}

	if dataFlags.SlaveAddress != "" {

		requestData.Address = dataFlags.SlaveAddress
		requestData.Role = "slave"
		requestData.MasterAddress = dataFlags.MasterAddress

	} else {
		requestData.Address = dataFlags.MasterAddress
		requestData.Role = "master"
	}

	encodedData, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	requestBody := bytes.NewBuffer(encodedData)

	setRequest, err := http.NewRequest("POST", requestURI, requestBody)
	if err != nil {
		return err
	}

	res, err := client.Do(setRequest)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var hashServerResponse response.RedisListTemplate
	decoder := json.NewDecoder(res.Body)

	if err := decoder.Decode(&hashServerResponse); err != nil {
		return err
	}

	fmt.Printf("  Add New Client (%s) 명령 수행 : \n", requestData.Role)
	fmt.Printf("    - 결과 : %s\n", hashServerResponse.Message)
	fmt.Printf("    - 현재 등록된 마스터 : \n")
	for i, eachMaster := range hashServerResponse.Masters {
		fmt.Printf("        %d) : %s\n", i+1, eachMaster.Address)
	}
	fmt.Printf("    - 현재 등록된 슬레이브 : \n")
	for i, eachSlave := range hashServerResponse.Slaves {
		fmt.Printf("        %d) : %s\n", i+1, eachSlave.Address)
	}
	return nil
}

func requestClientListToServer() error {

	requestURI := fmt.Sprintf("%s/clients", baseUrl)

	res, err := http.Get(requestURI)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var hashServerResponse response.RedisListTemplate
	decoder := json.NewDecoder(res.Body)

	if err := decoder.Decode(&hashServerResponse); err != nil {
		return err
	}

	fmt.Printf("  Get Client List 명령 수행 : \n")
	fmt.Printf("    - 현재 등록된 마스터 : \n")
	for i, eachMaster := range hashServerResponse.Masters {
		fmt.Printf("        %d) : %s\n", i+1, eachMaster.Address)
	}
	fmt.Printf("    - 현재 등록된 슬레이브 : \n")
	for i, eachSlave := range hashServerResponse.Slaves {
		fmt.Printf("        %d) : %s\n", i+1, eachSlave.Address)
	}

	return nil
}

func requestGetToServer(key string) error {
	requestURI := fmt.Sprintf("%s/hash/data/%s", baseUrl, key)

	res, err := http.Get(requestURI)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var hashServerResponse response.GetResultTemplate
	decoder := json.NewDecoder(res.Body)

	if err := decoder.Decode(&hashServerResponse); err != nil {
		return err
	}

	fmt.Printf("  Get %s 명령 수행 : \n", key)
	fmt.Printf("    - 결과 : %s\n", hashServerResponse.Result)
	fmt.Printf("    - 처리한 레디스 주소 : %s\n", hashServerResponse.NodeAdrress)

	return nil
}

func requestSetToServer(dataFlags DataFlag) error {

	requestURI := fmt.Sprintf("%s/hash/data", baseUrl)

	client := &http.Client{}

	KeyValue := storage.KeyValuePair{
		Key:   dataFlags.Key,
		Value: dataFlags.Value,
	}

	requestData := models.DataRequestContainer{}
	requestData.Data = append(requestData.Data, KeyValue)

	encodedData, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	requestBody := bytes.NewBuffer(encodedData)

	setRequest, err := http.NewRequest("POST", requestURI, requestBody)
	if err != nil {
		return err
	}

	res, err := client.Do(setRequest)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var hashServerResponse response.SetResultTemplate
	decoder := json.NewDecoder(res.Body)

	if err := decoder.Decode(&hashServerResponse); err != nil {
		return err
	}

	fmt.Printf("  Set %s 명령 수행 : \n", dataFlags.Key)
	fmt.Printf("    - 결과 : %s\n", hashServerResponse.Results[0].Result)
	fmt.Printf("    - 처리한 레디스 주소 : %s\n", hashServerResponse.Results[0].NodeAdrress)

	return nil
}

func requestNodeRegistration(clusterflag ClusterFlag) error {

	requestURI := fmt.Sprintf(
		"%s/api/v1/cluster?handshake=%t&startPoint=%t",
		baseUrl,
		clusterflag.Handshake,
		clusterflag.Startpoint,
	)

	newHost := cluster.Register{
		Address: clusterflag.Host,
	}

	encodedData, err := json.Marshal(newHost)
	if err != nil {
		return err
	}

	requestBody := bytes.NewBuffer(encodedData)

	req, err := http.NewRequest(
		http.MethodPost,
		requestURI,
		requestBody,
	)

	if err != nil {
		return err
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var response response.BasicTemplate
	decoder := json.NewDecoder(res.Body)

	if err := decoder.Decode(&response); err != nil {
		return err
	}

	if res.StatusCode > 400 {
		return fmt.Errorf(
			"클러스터 노드(%s) 등록 실패 - %s",
			clusterflag.Host,
			response.Message,
		)
	}

	fmt.Printf("  Cluster Node 등록 명령 수행 : \n")
	fmt.Printf(
		"    - 노드 (%s) 등록 성공 : \n",
		clusterflag.Host,
	)

	return nil
}

func requestClusterStart() error {

	requestURI := fmt.Sprintf(
		"%s/api/v1/cluster",
		baseUrl,
	)

	req, err := http.NewRequest(
		http.MethodPut,
		requestURI,
		nil,
	)
	if err != nil {
		return err
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var response response.BasicTemplate
	decoder := json.NewDecoder(res.Body)

	if err := decoder.Decode(&response); err != nil {
		return err
	}

	if res.StatusCode > 400 {
		return fmt.Errorf(
			"클러스터 시작 실패 : %s",
			response.Message,
		)
	}

	fmt.Printf("  Cluster 시작 성공! \n")

	return nil
}

func printRegisteredNodes() error {

	requestURI := fmt.Sprintf(
		"%s/api/v1/cluster",
		baseUrl,
	)

	req, err := http.NewRequest(
		http.MethodGet,
		requestURI,
		nil,
	)
	if err != nil {
		return err
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var response response.ClusterNodeListTemplate
	decoder := json.NewDecoder(res.Body)

	if err := decoder.Decode(&response); err != nil {
		return err
	}

	if res.StatusCode > 400 {
		return fmt.Errorf(
			"클러스터 등록 노드 가져오기 실패 : %s",
			response.Message,
		)
	}

	for _, eachNode := range response.Nodes {

		name, isSet := configs.ServerIpToDomainMap[eachNode]
		if !isSet {
			fmt.Printf("  등록된 노드 : %s \n", eachNode)
		} else {
			fmt.Printf("  등록된 노드 : %s \n", name)

		}
	}

	return nil
}

func printLeader() error {

	requestURI := fmt.Sprintf(
		"%s/api/v1/cluster/leader",
		baseUrl,
	)

	req, err := http.NewRequest(
		http.MethodGet,
		requestURI,
		nil,
	)
	if err != nil {
		return err
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var response response.BasicTemplate
	decoder := json.NewDecoder(res.Body)

	if err := decoder.Decode(&response); err != nil {
		return err
	}

	if res.StatusCode > 400 {
		return fmt.Errorf(
			"리더 가져오기 실패 : %s",
			response.Message,
		)
	}

	name, isSet := configs.ServerIpToDomainMap[response.Message]
	if !isSet {
		fmt.Printf("  현재 리더 : %s \n", response.Message)
		return nil
	}

	fmt.Printf("  현재 리더 : %s \n", name)

	return nil
}

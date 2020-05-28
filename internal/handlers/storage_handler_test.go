package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash_interface/configs"
	"hash_interface/internal/storage"
	"hash_interface/internal/models"
	"hash_interface/internal/models/response"
	"log"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

func TestMutex(t *testing.T) {

	ctx := context.Background()

	// Use Env For docker sdk client setup
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		println("docker client setup error", err)
		log.Fatal(err)

	}

	masterTwoContainer, err := getContainer(dockerClient, ctx, configs.RedisMasterTwoAddress)
	if err != nil {
		println("Get master one container error")
		log.Fatal(err)
	}

	slaveTwoContainer, err := getContainer(dockerClient, ctx, configs.RedisSlaveTwoAddress)
	if err != nil {
		println("Get slave one container error")
		log.Fatal(err)
	}

	masterThreeContainer, err := getContainer(dockerClient, ctx, configs.RedisMasterThreeAddress)
	if err != nil {
		println("Get master one container error")
		log.Fatal(err)
	}

	slaveThreeContainer, err := getContainer(dockerClient, ctx, configs.RedisSlaveThreeAddress)
	if err != nil {
		println("Get slave one container error")
		log.Fatal(err)
	}

	var dummyKeys = []string{"a", "b", "c", "d", "e" /*Kill*/, "f", "g", "h", "i", "j" /*Kill*/, "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}

	/* 값 저장하다가 컨테이너 죽이기 : <Master two, Slave two> 포트 8001번 를 중단
	 * 예상 결과 :
	 * 1) Master Two 가 죽었을 때(5번째 request 이후), Slave Two가 Handle
	 * 2) Slave Two 가 죽었을 때(10번째 request 이후), Master One (8000번 포트) or Master Three(8002번 포트)가 Handle
	 */
	for sendOrder, eachDummyKey := range dummyKeys {

		// if eachDummyKey == "h" || eachDummyKey == "l" {
		// 	requestGetValue("ag")
		// }

		requestSetKeyValue("a"+eachDummyKey, sendOrder)

		switch sendOrder {
		case 5:
			if err := dockerClient.ContainerStop(ctx, masterTwoContainer.ID, nil); err != nil {
				panic(err)
			}
			println(configs.RedisMasterTwoAddress, " Master two Killed!!")
			break
		case 10:
			if err := dockerClient.ContainerStop(ctx, slaveTwoContainer.ID, nil); err != nil {
				panic(err)
			}
			println(configs.RedisSlaveTwoAddress, " Slave two(Currently Master) Killed!!")
			break
		}
	}

	/* 값 가져오다가 컨테이너 죽이기 - <Master three, Slave three> 포트 8002번 을 중단
	* 예상 결과 : (이미 Master two, slave two는 죽어서 8001번 포트 컨테이너는 안 나와야한다.)
	 * 1) Master Three 가 죽었을 때(5번째 request 이후), Slave Three Handle
	 * 2) Slave Three 가 죽었을 때(10번째 request 이후), Master One (8000번 포트)먼아 Handle
	*/
	for sendOrder, eachDummyKey := range dummyKeys {

		requestGetValue("a" + eachDummyKey)

		switch sendOrder {
		case 5:
			if err := dockerClient.ContainerStop(ctx, masterThreeContainer.ID, nil); err != nil {
				panic(err)
			}
			println(configs.RedisMasterThreeAddress, " Master three Killed!!")
			break
		case 10:
			if err := dockerClient.ContainerStop(ctx, slaveThreeContainer.ID, nil); err != nil {
				panic(err)
			}
			println(configs.RedisSlaveThreeAddress, " Slave three(Currently Master) Killed!!")
			break
		}
	}

}

func requestSetKeyValue(key string, value int) {

	var requestData models.DataRequestContainer
	requestData.Data = make([]cluster.KeyValuePair, 1)
	requestData.Data[0] = cluster.KeyValuePair{
		Key:   key,
		Value: strconv.Itoa(value),
	}

	// requestBodyInString := fmt.Sprintf(requestDataFormat, key, "dummy"+strconv.Itoa(value))

	requestDataInString, err := json.Marshal(requestData)
	if err != nil {
		println("Errors in marshal")
		log.Fatal(err)
	}

	var requestBody bytes.Buffer
	requestBody.Write(requestDataInString)

	reqeustToInterface, err := http.NewRequest(http.MethodPost, "http://localhost:8001/hash/data", &requestBody)
	if err != nil {
		println("Errors in request")
		log.Fatal(err)
	}
	reqeustToInterface.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(reqeustToInterface)
	if err != nil {
		println("Error in response of interfact ", err)
	}

	if res.StatusCode >= 400 {
		println("Response error!!")
		return
	}

	var responseContainer response.SetResultTemplate
	decoder := json.NewDecoder(res.Body)
	if err := decoder.Decode(&responseContainer); err != nil {
		println("Response json decod error")
		log.Fatal(err)
	}

	for _, eachResponseData := range responseContainer.Results {
		fmt.Printf("SET 요청 결과 : %s 노드에서 (key, value) = (%s) 처리\n", eachResponseData.NodeAdrress, eachResponseData.Result)
	}
}

func requestGetValue(key string) {

	var requestBody bytes.Buffer

	reqeustToInterface, err := http.NewRequest(http.MethodGet, "http://localhost:8001/hash/data/"+key, &requestBody)
	if err != nil {
		println("Errors in GET request")
		log.Fatal(err)
	}

	client := &http.Client{}
	response, err := client.Do(reqeustToInterface)
	if err != nil {
		println("Error in GET response of interfact ", err)
	}

	if response.StatusCode >= 400 {
		println("Get ", key, "Response error!!")
		return
	}

	type GetResponseFormat struct {
		Response   string `json:"response"`
		HandleNode string `json:"handle_node"`
	}

	// responseDataByte, _ := ioutil.ReadAll(response.Body)

	var responseContainer GetResponseFormat
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&responseContainer); err != nil {
		println("Response json decod error")
		log.Fatal(err)
	}

	fmt.Printf("GET %s 요청 결과 : %s 노드에서 %s 받아옴\n", key, responseContainer.HandleNode, responseContainer.Response)
}

func getContainer(dockerClient *client.Client, dockerContext context.Context, address string) (types.Container, error) {

	var err error

	containers, err := dockerClient.ContainerList(dockerContext, types.ContainerListOptions{})
	if err != nil {
		return types.Container{}, err
	}

	for _, eachContainer := range containers {
		// println("GetContainer() : each container - ", eachContainer.NetworkSettings.Networks)
		for _, endPointSetting := range eachContainer.NetworkSettings.Networks {
			if strings.Contains(address, endPointSetting.IPAddress) {
				return eachContainer, nil
			}

		}
	}

	return types.Container{}, fmt.Errorf("No Such Container with IP : %s", address)
}

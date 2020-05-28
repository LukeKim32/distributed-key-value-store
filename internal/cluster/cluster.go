package cluster

import (
	"fmt"
	"os"
)

type ClusterInfo struct {
	curIpAddress string

	nodeAddressList []string

	addressRegisterCheck map[string]bool
}

const (
	HandShake   = true
	NoHandShake = false

	// StartPoint : 클러스터 등록의 시작 지점인지
	StartPoint   = true
	NoStartPoint = false
)

func (this *ClusterInfo) Init() error {

	this.curIpAddress = os.Getenv("DOCKER_HOST_IP")
	this.addressRegisterCheck = make(map[string]bool)

	return nil
}

func (this StateMachine) GetCurServerIP() string {
	return this.Cluster.curIpAddress
}

func (this StateMachine) GetNodeAddressList() []string {
	return this.Cluster.nodeAddressList
}

func (this StateMachine) IsStartable() error {

	if len(this.Cluster.nodeAddressList) < 2 {
		return fmt.Errorf(
			"현재 노드를 제외한 등록된 노드의 개수가 2개 이상이어야 합니다",
		)
	}

	if len(this.Cluster.nodeAddressList)%2 != 0 {
		return fmt.Errorf(
			"현재 노드를 제외한 등록된 노드는 짝수개이어야 합니다",
		)
	}

	return nil
}

func (this *StateMachine) AddNewNode(address string) {

	_, isRegistered := this.Cluster.addressRegisterCheck[address]
	if isRegistered == false {
		this.Cluster.addressRegisterCheck[address] = true
		this.Cluster.nodeAddressList = append(
			this.Cluster.nodeAddressList,
			address,
		)
	}

}

func (this *StateMachine) DeleteNode(address string) error {

	_, isRegistered := this.Cluster.addressRegisterCheck[address]
	if isRegistered == false {
		return fmt.Errorf(
			"클러스터에 등록되지 않은 노드(%s)입니다\n",
			address,
		)
	}

	delete(this.Cluster.addressRegisterCheck, address)

	newAddressList := []string{}

	for _, eachAddress := range this.Cluster.nodeAddressList {
		if eachAddress != address {
			newAddressList = append(newAddressList, eachAddress)
		}
	}

	this.Cluster.nodeAddressList = newAddressList

	return nil
}

func (this *StateMachine) Register(newHost string) error {

	_, isRegistered := this.Cluster.addressRegisterCheck[newHost]
	if isRegistered {
		return fmt.Errorf(
			"클러스터에 이미 등록된 노드(%s)입니다\n",
			newHost,
		)
	}

	for _, eachAddress := range this.Cluster.nodeAddressList {

		err := this.SendRegisterMsg(
			eachAddress,
			newHost,
			HandShake,
			NoStartPoint,
		)

		if err != nil {
			return err
		}
	}

	err := this.SendRegisterMsg(
		newHost,
		this.Cluster.curIpAddress,
		NoHandShake,
		NoStartPoint,
	)

	if err != nil {
		return err
	}

	return nil
}

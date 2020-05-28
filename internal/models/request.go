package models

import (
	"hash_interface/internal/storage"
)

type DataRequestContainer struct {
	Data []storage.KeyValuePair `json:"data"`
}

type WalMsgContainer struct {
	Data map[uint64]storage.KeyValuePair `json:"data"`
}

type NewClientRequestContainer struct {
	// Address : 레디스 노드 주소, IP + Port
	Address string `json:"address"`

	// Role : "Master" / "Slave"
	Role string `json:"role"`

	// MasterAddress : Role = "Slave" 일 때 반드시 필요한 옵션 (Role = "Master" 일 경우 무시)
	MasterAddress string `json:"master_address"`
}

func (client NewClientRequestContainer) IsEmpty() bool {
	if client.Address == "" {
		return true
	}
	if client.Role == "" {
		return true
	}

	return false
}

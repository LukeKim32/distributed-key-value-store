package response

import (
	"encoding/json"
	"hash_interface/internal/storage"
)

type ResponseTemplate interface {
	Marshal(curMsg, nextMsg, nextLink string) ([]byte, error)
}

type BasicTemplate struct {
	Message  string   `json:"message"`
	NextLink NextLink `json:"next_link"`
}

type NextLink struct {
	Message string `json:"message"`
	Href    string `json:"href`
}

type RedisListTemplate struct {
	Masters []storage.RedisClient `json:"masters"`
	Slaves  []storage.RedisClient `json:"slaves"`
	BasicTemplate
}

type ClusterNodeListTemplate struct {
	Nodes []string
	BasicTemplate
}

// MarshalBasicTemplate marshals Response template struct into JSON byte with passed @messages
func (template BasicTemplate) Marshal(curMsg, nextMsg, nextLink string) ([]byte, error) {

	template.Message = curMsg
	template.NextLink.Message = nextMsg
	template.NextLink.Href = nextLink

	// JSON marshaling(Encoding to Bytes)
	encodedTemplate, err := json.Marshal(template)
	if err != nil {
		return nil, err
	}

	return encodedTemplate, nil
}

func (template RedisListTemplate) Marshal(curMsg, nextMsg, nextLink string) ([]byte, error) {

	template.Message = curMsg
	template.NextLink.Message = nextMsg
	template.NextLink.Href = nextLink

	encodedTemplate, err := json.Marshal(template)
	if err != nil {
		return nil, err
	}

	return encodedTemplate, nil
}

func (template ClusterNodeListTemplate) Marshal(
	Nodes []string,
	curMsg, nextMsg, nextLink string,
) ([]byte, error) {

	template.Nodes = Nodes
	template.Message = curMsg
	template.NextLink.Message = nextMsg
	template.NextLink.Href = nextLink

	encodedTemplate, err := json.Marshal(template)
	if err != nil {
		return nil, err
	}

	return encodedTemplate, nil
}

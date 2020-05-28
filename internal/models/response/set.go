package response

import (
	"encoding/json"
)

type RedisResult struct {
	Result      string `json:"result"`
	NodeAdrress string `json:"handled_node"`
}

type SetResultTemplate struct {
	Results []RedisResult `json:"results"`
	BasicTemplate
}

func (template SetResultTemplate) Marshal(curMsg, nextMsg, nextLink string) ([]byte, error) {

	template.Message = curMsg
	template.NextLink.Message = nextMsg
	template.NextLink.Href = nextLink

	encodedTemplate, err := json.Marshal(template)
	if err != nil {
		return nil, err
	}

	return encodedTemplate, nil
}

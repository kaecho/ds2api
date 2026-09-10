package smid

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed js/sm_conf.json
var confJSON []byte

type confuseRule struct {
	Cipher         string `json:"cipher"`
	IsEncrypt      int    `json:"is_encrypt"`
	Key            string `json:"key"`
	ObfuscatedName string `json:"obfuscated_name"`
}

type smConf struct {
	Protocol      int `json:"Protocol"`
	ConfusionInfo struct {
		Data map[string]confuseRule `json:"data"`
	} `json:"ConfusionInfo"`
}

var (
	confOnce   sync.Once
	parsedConf smConf
	confErr    error
)

func loadConf() (smConf, error) {
	confOnce.Do(func() {
		if err := json.Unmarshal(confJSON, &parsedConf); err != nil {
			confErr = fmt.Errorf("sm_conf.json: %w", err)
		}
	})
	return parsedConf, confErr
}

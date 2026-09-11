package llmmodel

import (
	"react-base-service/components/params"
	"react-base-service/conf"
)

// IsWhitelisted 判断用户是否在白名单中
func IsWhitelisted(userName string) bool {
	for _, u := range conf.CustomConf.LLM.ModelWhitelist {
		if u == userName {
			return true
		}
	}
	return false
}

// GetWhitelist 返回白名单用户列表
func GetWhitelist() params.WhitelistResp {
	users := conf.CustomConf.LLM.ModelWhitelist
	if users == nil {
		users = []string{}
	}
	return params.WhitelistResp{Users: users}
}

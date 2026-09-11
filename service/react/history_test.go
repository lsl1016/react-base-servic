package react

import (
	"testing"

	"react-base-service/conf"
)

func TestCanViewReactSessionHistory(t *testing.T) {
	originalWhitelist := conf.CustomConf.LLM.React.PlaygroundWhitelist
	defer func() { conf.CustomConf.LLM.React.PlaygroundWhitelist = originalWhitelist }()
	conf.CustomConf.LLM.React.PlaygroundWhitelist = []string{"alice"}

	tests := []struct {
		name            string
		loginUserName   string
		sessionUserName string
		want            bool
	}{
		{name: "本人查看自己会话且不在白名单", loginUserName: "bob", sessionUserName: "bob", want: true},
		{name: "本人比对忽略首尾空格", loginUserName: " bob ", sessionUserName: "bob", want: true},
		{name: "白名单用户查看他人会话", loginUserName: "alice", sessionUserName: "bob", want: true},
		{name: "非白名单用户查看他人会话", loginUserName: "carol", sessionUserName: "bob", want: false},
		{name: "登录用户为空且会话归属为空", loginUserName: "", sessionUserName: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canViewReactSessionHistory(tt.loginUserName, tt.sessionUserName); got != tt.want {
				t.Fatalf("canViewReactSessionHistory(%q, %q) = %v, want %v",
					tt.loginUserName, tt.sessionUserName, got, tt.want)
			}
		})
	}
}

func TestCanViewReactSessionHistoryAllowsEveryoneWhenWhitelistEmpty(t *testing.T) {
	originalWhitelist := conf.CustomConf.LLM.React.PlaygroundWhitelist
	defer func() { conf.CustomConf.LLM.React.PlaygroundWhitelist = originalWhitelist }()
	conf.CustomConf.LLM.React.PlaygroundWhitelist = nil

	if !canViewReactSessionHistory("carol", "bob") {
		t.Fatalf("白名单为空时应不限制跨用户查看")
	}
}

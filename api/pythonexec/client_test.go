package pythonexec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"react-base-service/conf"

	"react-base-service/golib/base"
)

func TestExecute(t *testing.T) {
	if err := os.MkdirAll("log", 0o755); err != nil {
		t.Fatalf("MkdirAll log failed: %v", err)
	}

	tests := []struct {
		name string
		body string
	}{
		{
			name: "code_200_success",
			body: `{"msg":"操作成功","code":200,"data":{"exitCode":0,"stdout":"{\"structuredResult\":{\"metrics\":{\"total\":1}},\"artifacts\":[]}","stderr":"","timedOut":false}}`,
		},
		{
			name: "code_0_success",
			body: `{"message":"success","code":0,"data":{"exitCode":0,"stdout":"{\"structuredResult\":{\"metrics\":{\"total\":1}},\"artifacts\":[]}","stderr":"","timedOut":false}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			oldCfg := conf.API.PythonExec
			conf.API.PythonExec = base.ApiClient{
				Domain:  server.URL,
				Timeout: 3 * time.Second,
			}
			defer func() {
				conf.API.PythonExec = oldCfg
			}()

			resp, err := Execute(context.Background(), &ExecuteRequest{
				LogID:  "log-1",
				Python: "print('ok')",
				Data:   "{}",
			})
			if err != nil {
				t.Fatalf("Execute returned err=%v", err)
			}
			if resp.ExitCode != 0 || resp.TimedOut {
				t.Fatalf("unexpected response: %#v", resp)
			}
			if resp.Stdout == "" {
				t.Fatalf("expected stdout to be populated")
			}
		})
	}
}

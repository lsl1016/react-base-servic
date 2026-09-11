package cos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	tencentcos "github.com/tencentyun/cos-go-sdk-v5"
)

func TestBuildBucketHost(t *testing.T) {
	tests := []struct {
		name     string
		bucket   string
		appID    string
		endpoint string
		region   string
		want     string
		wantErr  bool
	}{
		{
			name:     "bucket already contains appid",
			bucket:   "my-app-bucket-1253445850",
			appID:    "1253445850",
			endpoint: "cos.ap-beijing.myqcloud.com",
			want:     "my-app-bucket-1253445850.cos.ap-beijing.myqcloud.com",
		},
		{
			name:     "append appid when missing",
			bucket:   "my-app-bucket",
			appID:    "1253445850",
			endpoint: "cos.ap-beijing.myqcloud.com",
			want:     "my-app-bucket-1253445850.cos.ap-beijing.myqcloud.com",
		},
		{
			name:   "fallback to region",
			bucket: "demo-bucket",
			appID:  "123456",
			region: "ap-beijing",
			want:   "demo-bucket-123456.cos.ap-beijing.myqcloud.com",
		},
		{
			name:    "missing endpoint and region",
			bucket:  "demo-bucket",
			appID:   "123456",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildBucketHost(tc.bucket, tc.appID, tc.endpoint, tc.region)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("host mismatch, want=%s got=%s", tc.want, got)
			}
		})
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	tests := map[string]string{
		"https://cos.ap-beijing.myqcloud.com": "cos.ap-beijing.myqcloud.com",
		"cos.ap-beijing.myqcloud.com":         "cos.ap-beijing.myqcloud.com",
	}

	for input, want := range tests {
		got := normalizeEndpoint(input)
		if got != want {
			t.Fatalf("normalize endpoint failed, input=%s want=%s got=%s", input, want, got)
		}
	}
}

func TestDownloadRangeUsesHTTPRangeAndValidatesLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Range") != "bytes=2-5" {
			http.Error(writer, "unexpected range", http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Range", "bytes 2-5/8")
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write([]byte("cdef"))
	}))
	defer server.Close()
	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{raw: tencentcos.NewClient(&tencentcos.BaseURL{BucketURL: baseURL}, server.Client())}

	data, err := client.DownloadRange(context.Background(), "document.md", 2, 5)
	if err != nil {
		t.Fatalf("download range: %v", err)
	}
	if string(data) != "cdef" {
		t.Fatalf("unexpected range content: %q", data)
	}
	if _, err := client.DownloadRange(context.Background(), "document.md", 5, 2); err == nil {
		t.Fatal("expected invalid range error")
	}
}

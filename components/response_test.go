package components

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRenderJsonFailMapsPermissionErrorsToForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantErrNo  int
	}{
		{
			name:       "react playground forbidden",
			err:        ErrorReactPlaygroundForbidden,
			wantStatus: http.StatusForbidden,
			wantErrNo:  ErrorReactPlaygroundForbidden.ErrNo,
		},
		{
			name:       "existing errors remain http 200",
			err:        ErrorParamInvalid.Sprintf("参数错误"),
			wantStatus: http.StatusOK,
			wantErrNo:  ErrorParamInvalid.ErrNo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/test", nil)

			RenderJsonFail(ctx, tt.err)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			var body DefaultRenderWithTrace
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.ErrNo != tt.wantErrNo {
				t.Fatalf("errNo = %d, want %d", body.ErrNo, tt.wantErrNo)
			}
		})
	}
}

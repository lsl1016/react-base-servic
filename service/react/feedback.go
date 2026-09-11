package react

import (
	"strings"
	"time"

	"react-base-service/components"
	"react-base-service/components/params"
	"react-base-service/helpers"
	model "react-base-service/models/llm"

	"github.com/gin-gonic/gin"
)

const reactProblemFeedbackMaxRunes = 500

// UpdateReactRunFeedback 更新某一轮(run)的点赞/点踩与问题反馈。
func UpdateReactRunFeedback(ctx *gin.Context, req *params.ReactRunFeedbackReq) (*params.ReactRunFeedbackResp, error) {
	if req == nil {
		return nil, components.ErrorParamInvalid.Sprintf("请求体不能为空")
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		return nil, components.ErrorParamInvalid.Sprintf("runId不能为空")
	}
	if req.Feedback == nil && req.ProblemFeedback == nil {
		return nil, components.ErrorParamInvalid.Sprintf("feedback和problemFeedback至少传一个")
	}

	var feedbackPtr *int
	if req.Feedback != nil {
		value, ok := parseReactRunFeedback(*req.Feedback)
		if !ok {
			return nil, components.ErrorParamInvalid.Sprintf("feedback必须为 1、-1 或 0")
		}
		feedbackPtr = &value
	}

	var problemPtr *string
	if req.ProblemFeedback != nil {
		problem := strings.TrimSpace(*req.ProblemFeedback)
		if len([]rune(problem)) > reactProblemFeedbackMaxRunes {
			return nil, components.ErrorParamInvalid.Sprintf("problemFeedback长度不能超过500个字符")
		}
		problemPtr = &problem
	}

	userName := helpers.GetUserName(ctx)
	if userName == "" {
		return nil, components.ErrorUserNameMismatch
	}

	run, err := model.GetReactRunByRunID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, components.ErrorParamInvalid.Sprintf("runId无效")
	}
	if run.UserName != userName {
		return nil, components.ErrorUserNameMismatch
	}

	fb, err := model.UpsertReactRunFeedback(ctx, runID, run.SessionID, userName, feedbackPtr, problemPtr)
	if err != nil {
		return nil, err
	}
	return toReactRunFeedbackResp(fb), nil
}

// ListReactSessionFeedback 返回某会话下所有轮次的反馈，用于历史回显。
func ListReactSessionFeedback(ctx *gin.Context, req *params.ReactSessionFeedbackReq) (*params.ReactSessionFeedbackResp, error) {
	if req == nil {
		return nil, components.ErrorParamInvalid.Sprintf("请求体不能为空")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return nil, components.ErrorParamInvalid.Sprintf("sessionId不能为空")
	}

	session, err := model.GetReactSessionBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, components.ErrorReactSessionNotFound.Sprintf(sessionID)
	}
	if session.UserName != helpers.GetUserName(ctx) {
		return nil, components.ErrorUserNameMismatch
	}

	list, err := model.GetReactRunFeedbacksBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	items := make([]params.ReactRunFeedbackResp, 0, len(list))
	for i := range list {
		items = append(items, *toReactRunFeedbackResp(&list[i]))
	}
	return &params.ReactSessionFeedbackResp{Items: items}, nil
}

func parseReactRunFeedback(value int) (int, bool) {
	switch value {
	case model.ReactRunFeedbackLike, model.ReactRunFeedbackDislike, model.ReactRunFeedbackNone:
		return value, true
	default:
		return model.ReactRunFeedbackNone, false
	}
}

func formatReactFeedbackTime(t *time.Time) string {
	// nil、零值或列默认 '1970-01-01 00:00:00'（Unix <= 0）都视为「未设置」
	if t == nil || t.IsZero() || t.Unix() <= 0 {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func toReactRunFeedbackResp(fb *model.ReactRunFeedback) *params.ReactRunFeedbackResp {
	return &params.ReactRunFeedbackResp{
		RunID:                    fb.RunID,
		SessionID:                fb.SessionID,
		Feedback:                 fb.Feedback,
		ProblemFeedback:          fb.ProblemFeedback,
		FeedbackUpdatedAt:        formatReactFeedbackTime(fb.FeedbackUpdatedAt),
		ProblemFeedbackUpdatedAt: formatReactFeedbackTime(fb.ProblemFeedbackUpdatedAt),
	}
}

package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"react-base-service/conf"
)

type MiniMaxClient struct {
	apiKey       string
	defaultModel string
	config       conf.EndpointConfig
}

func NewMiniMaxClient(apiKey, defaultModel string, config conf.EndpointConfig) *MiniMaxClient {
	return &MiniMaxClient{apiKey: apiKey, defaultModel: defaultModel, config: config}
}

type minimaxStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type minimaxRequest struct {
	Model               string                `json:"model"`
	MaxCompletionTokens int                   `json:"max_completion_tokens,omitempty"`
	Stream              bool                  `json:"stream"`
	StreamOptions       *minimaxStreamOptions `json:"stream_options,omitempty"`
	Messages            []LLMMessage          `json:"messages"`
}

type minimaxFileRequest struct {
	Model               string                `json:"model"`
	MaxCompletionTokens int                   `json:"max_completion_tokens,omitempty"`
	Stream              bool                  `json:"stream"`
	StreamOptions       *minimaxStreamOptions `json:"stream_options,omitempty"`
	Messages            []minimaxFileMessage  `json:"messages"`
}

type minimaxFileMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type minimaxContentBlock struct {
	Type string             `json:"type"`
	Text string             `json:"text,omitempty"`
	File *minimaxFileSource `json:"file,omitempty"`
}

type minimaxFileSource struct {
	FileData string `json:"file_data"`
	FileName string `json:"filename,omitempty"`
}

type minimaxStreamChunk struct {
	Choices []minimaxChoice `json:"choices"`
	Usage   *minimaxUsage   `json:"usage,omitempty"`
}

type minimaxChoice struct {
	Delta        minimaxDelta `json:"delta"`
	FinishReason *string      `json:"finish_reason"`
}

type minimaxDelta struct {
	Content          string                 `json:"content"`
	ReasoningContent string                 `json:"reasoning_content,omitempty"`
	Reasoning        string                 `json:"reasoning,omitempty"`
	ToolCalls        []minimaxToolCallDelta `json:"tool_calls,omitempty"`
}

type minimaxToolDef struct {
	Type     string          `json:"type"`
	Function minimaxFunction `json:"function"`
}

type minimaxFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type minimaxToolCallDelta struct {
	Index    int               `json:"index"`
	ID       string            `json:"id,omitempty"`
	Type     string            `json:"type,omitempty"`
	Function *minimaxFuncDelta `json:"function,omitempty"`
}

type minimaxFuncDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type minimaxToolRequest struct {
	Model               string                `json:"model"`
	MaxCompletionTokens int                   `json:"max_completion_tokens,omitempty"`
	Stream              bool                  `json:"stream"`
	StreamOptions       *minimaxStreamOptions `json:"stream_options,omitempty"`
	Tools               []minimaxToolDef      `json:"tools,omitempty"`
	ToolChoice          ToolChoice            `json:"tool_choice,omitempty"`
	Messages            []minimaxAnyMsg       `json:"messages"`
}

type minimaxAnyMsg struct {
	Role       string               `json:"role"`
	Content    interface{}          `json:"content"`
	ToolCalls  []minimaxToolCallMsg `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

type minimaxToolCallMsg struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function minimaxFuncCall `json:"function"`
}

type minimaxFuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type minimaxUsage struct {
	CompletionTokens    int                        `json:"completion_tokens"`
	PromptTokens        int                        `json:"prompt_tokens"`
	TotalTokens         int                        `json:"total_tokens"`
	PromptTokensDetails minimaxPromptTokensDetails `json:"prompt_tokens_details"`
}

type minimaxPromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

func toMiniMaxTools(tools []ToolDefinition) []minimaxToolDef {
	out := make([]minimaxToolDef, len(tools))
	for i, t := range tools {
		out[i] = minimaxToolDef{
			Type: "function",
			Function: minimaxFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}
	return out
}

func (m *MiniMaxClient) ChatStream(ctx context.Context, messages []LLMMessage, model string) (<-chan StreamChunk, error) {
	if model == "" {
		model = m.defaultModel
	}

	maxTokens := m.config.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	reqBody := minimaxRequest{
		Model:               model,
		Messages:            messages,
		MaxCompletionTokens: maxTokens,
		Stream:              true,
		StreamOptions:       &minimaxStreamOptions{IncludeUsage: true},
	}

	bodyBytes, err := marshalRequestBodyGuard(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := m.config.ApiUrl + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	applyTraceHeadersFromContext(ctx, req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("minimax api error (status %d): %s", resp.StatusCode, string(body))
	}

	return m.streamChatCompletionResponse(ctx, resp.Body), nil
}

func (m *MiniMaxClient) ChatStreamWithFilePayloads(
	ctx context.Context,
	messages []LLMMessage,
	model string,
	files []FilePayload,
) (<-chan StreamChunk, error) {
	if len(files) == 0 {
		return m.ChatStream(ctx, messages, model)
	}
	if model == "" {
		model = m.defaultModel
	}

	maxTokens := m.config.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	apiMessages, err := buildMiniMaxFileMessages(messages, files)
	if err != nil {
		return nil, err
	}
	reqBody := minimaxFileRequest{
		Model:               model,
		Messages:            apiMessages,
		MaxCompletionTokens: maxTokens,
		Stream:              true,
		StreamOptions:       &minimaxStreamOptions{IncludeUsage: true},
	}

	bodyBytes, err := marshalRequestBodyGuard(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := m.config.ApiUrl + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	applyTraceHeadersFromContext(ctx, req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("minimax api error (status %d): %s", resp.StatusCode, string(body))
	}

	return m.streamChatCompletionResponse(ctx, resp.Body), nil
}

func (m *MiniMaxClient) streamChatCompletionResponse(ctx context.Context, body io.ReadCloser) <-chan StreamChunk {
	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer body.Close()

		var (
			inputTokens       int
			outputTokens      int
			cacheReadTokens   int
			cacheCreateTokens int
			receivedDone      bool
			finishReason      string
			chunkCount        int
			lastChunkAt       time.Time
			terminationReason = StreamTerminationCompleted
		)

		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				terminationReason = StreamTerminationCancelled
				logStreamFinal(ctx, streamFinalState{
					provider:          "MiniMax",
					terminationReason: terminationReason,
					receivedDone:      receivedDone,
					finishReason:      finishReason,
					chunkCount:        chunkCount,
					inputTokens:       inputTokens,
					outputTokens:      outputTokens,
					cacheReadTokens:   cacheReadTokens,
					cacheCreateTokens: cacheCreateTokens,
					lastChunkAt:       lastChunkAt,
				})
				ch <- StreamChunk{
					Done:              true,
					Cancelled:         true,
					InputTokens:       inputTokens,
					OutputTokens:      outputTokens,
					CacheReadTokens:   cacheReadTokens,
					CacheCreateTokens: cacheCreateTokens,
					ReceivedDone:      receivedDone,
					FinishReason:      finishReason,
					TerminationReason: terminationReason,
				}
				return
			default:
			}

			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				receivedDone = true
				lastChunkAt = time.Now()
				if finishReason != "" && !isAcceptedFinishReason(finishReason) {
					terminationReason = finishReasonTerminationReason(finishReason)
					logStreamFinal(ctx, streamFinalState{
						provider:          "MiniMax",
						terminationReason: terminationReason,
						receivedDone:      receivedDone,
						finishReason:      finishReason,
						chunkCount:        chunkCount,
						inputTokens:       inputTokens,
						outputTokens:      outputTokens,
						cacheReadTokens:   cacheReadTokens,
						cacheCreateTokens: cacheCreateTokens,
						lastChunkAt:       lastChunkAt,
					})
					ch <- StreamChunk{
						Done:              true,
						Error:             fmt.Errorf("minimax stream finished with unacceptable finish_reason=%s", finishReason),
						InputTokens:       inputTokens,
						OutputTokens:      outputTokens,
						CacheReadTokens:   cacheReadTokens,
						CacheCreateTokens: cacheCreateTokens,
						ReceivedDone:      receivedDone,
						FinishReason:      finishReason,
						TerminationReason: terminationReason,
					}
					return
				}

				logStreamFinal(ctx, streamFinalState{
					provider:          "MiniMax",
					terminationReason: terminationReason,
					receivedDone:      receivedDone,
					finishReason:      finishReason,
					chunkCount:        chunkCount,
					inputTokens:       inputTokens,
					outputTokens:      outputTokens,
					cacheReadTokens:   cacheReadTokens,
					cacheCreateTokens: cacheCreateTokens,
					lastChunkAt:       lastChunkAt,
				})
				ch <- StreamChunk{
					Done:              true,
					InputTokens:       inputTokens,
					OutputTokens:      outputTokens,
					CacheReadTokens:   cacheReadTokens,
					CacheCreateTokens: cacheCreateTokens,
					ReceivedDone:      receivedDone,
					FinishReason:      finishReason,
					TerminationReason: terminationReason,
				}
				return
			}

			chunkCount++
			lastChunkAt = time.Now()

			var chunk minimaxStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				terminationReason = StreamTerminationUpstreamParseError
				logStreamParseError(ctx, "MiniMax", chunkCount, data, err)
				logStreamFinal(ctx, streamFinalState{
					provider:          "MiniMax",
					terminationReason: terminationReason,
					receivedDone:      receivedDone,
					finishReason:      finishReason,
					chunkCount:        chunkCount,
					inputTokens:       inputTokens,
					outputTokens:      outputTokens,
					cacheReadTokens:   cacheReadTokens,
					cacheCreateTokens: cacheCreateTokens,
					lastChunkAt:       lastChunkAt,
				})
				ch <- StreamChunk{
					Done:              true,
					Error:             fmt.Errorf("parse stream chunk: %w", err),
					InputTokens:       inputTokens,
					OutputTokens:      outputTokens,
					CacheReadTokens:   cacheReadTokens,
					CacheCreateTokens: cacheCreateTokens,
					ReceivedDone:      receivedDone,
					FinishReason:      finishReason,
					TerminationReason: terminationReason,
				}
				return
			}

			if chunk.Usage != nil {
				inputTokens = chunk.Usage.PromptTokens
				outputTokens = chunk.Usage.CompletionTokens
				cacheReadTokens = chunk.Usage.PromptTokensDetails.CachedTokens
			}

			for _, choice := range chunk.Choices {
				if choice.FinishReason != nil {
					finishReason = strings.TrimSpace(*choice.FinishReason)
				}
				if reasoning := firstNonEmpty(choice.Delta.ReasoningContent, choice.Delta.Reasoning); reasoning != "" {
					ch <- StreamChunk{ReasoningContent: reasoning}
				}
				if choice.Delta.Content != "" {
					ch <- StreamChunk{Content: choice.Delta.Content}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			terminationReason = StreamTerminationUpstreamReadError
			logStreamFinal(ctx, streamFinalState{
				provider:          "MiniMax",
				terminationReason: terminationReason,
				receivedDone:      receivedDone,
				finishReason:      finishReason,
				scannerErr:        err,
				chunkCount:        chunkCount,
				inputTokens:       inputTokens,
				outputTokens:      outputTokens,
				cacheReadTokens:   cacheReadTokens,
				cacheCreateTokens: cacheCreateTokens,
				lastChunkAt:       lastChunkAt,
			})
			ch <- StreamChunk{
				Done:              true,
				Error:             fmt.Errorf("read stream: %w", err),
				InputTokens:       inputTokens,
				OutputTokens:      outputTokens,
				CacheReadTokens:   cacheReadTokens,
				CacheCreateTokens: cacheCreateTokens,
				ReceivedDone:      receivedDone,
				FinishReason:      finishReason,
				TerminationReason: terminationReason,
			}
			return
		}

		if finishReason != "" && isAcceptedFinishReason(finishReason) {
			logStreamFinal(ctx, streamFinalState{
				provider:          "MiniMax",
				terminationReason: terminationReason,
				receivedDone:      receivedDone,
				finishReason:      finishReason,
				chunkCount:        chunkCount,
				inputTokens:       inputTokens,
				outputTokens:      outputTokens,
				cacheReadTokens:   cacheReadTokens,
				cacheCreateTokens: cacheCreateTokens,
				lastChunkAt:       lastChunkAt,
			})
			ch <- StreamChunk{
				Done:              true,
				InputTokens:       inputTokens,
				OutputTokens:      outputTokens,
				CacheReadTokens:   cacheReadTokens,
				CacheCreateTokens: cacheCreateTokens,
				ReceivedDone:      receivedDone,
				FinishReason:      finishReason,
				TerminationReason: terminationReason,
			}
			return
		}

		terminationReason = StreamTerminationUpstreamEOFWithoutDone
		if finishReason != "" && !isAcceptedFinishReason(finishReason) {
			terminationReason = finishReasonTerminationReason(finishReason)
		}
		logStreamFinal(ctx, streamFinalState{
			provider:          "MiniMax",
			terminationReason: terminationReason,
			receivedDone:      receivedDone,
			finishReason:      finishReason,
			chunkCount:        chunkCount,
			inputTokens:       inputTokens,
			outputTokens:      outputTokens,
			cacheReadTokens:   cacheReadTokens,
			cacheCreateTokens: cacheCreateTokens,
			lastChunkAt:       lastChunkAt,
		})
		ch <- StreamChunk{
			Done:              true,
			Error:             fmt.Errorf("stream ended without a valid completion signal"),
			InputTokens:       inputTokens,
			OutputTokens:      outputTokens,
			CacheReadTokens:   cacheReadTokens,
			CacheCreateTokens: cacheCreateTokens,
			ReceivedDone:      receivedDone,
			FinishReason:      finishReason,
			TerminationReason: terminationReason,
		}
	}()

	return ch
}

func buildMiniMaxFileMessages(messages []LLMMessage, files []FilePayload) ([]minimaxFileMessage, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages is empty")
	}
	targetIdx := minimaxFindLatestUserMessageIndex(messages)
	if targetIdx < 0 {
		targetIdx = len(messages) - 1
	}

	out := make([]minimaxFileMessage, 0, len(messages))
	for idx, msg := range messages {
		if idx != targetIdx {
			out = append(out, minimaxFileMessage{Role: msg.Role, Content: msg.Content})
			continue
		}

		blocks := make([]minimaxContentBlock, 0, len(files)+1)
		if strings.TrimSpace(msg.Content) != "" {
			blocks = append(blocks, minimaxContentBlock{Type: "text", Text: msg.Content})
		}
		for _, f := range files {
			if strings.EqualFold(strings.TrimSpace(f.ContentAs), "text") || strings.EqualFold(strings.TrimSpace(f.Encoding), "text") {
				if strings.TrimSpace(f.Text) == "" {
					return nil, fmt.Errorf("minimax text file content is empty, file=%s", f.FileName)
				}
				blocks = append(blocks, minimaxContentBlock{
					Type: "text",
					Text: buildMiniMaxTextFileBlock(f.FileName, f.Text),
				})
				continue
			}
			if strings.ToLower(strings.TrimSpace(f.Encoding)) != "base64" {
				return nil, fmt.Errorf("minimax file encoding only supports base64 for non-text file, file=%s", f.FileName)
			}
			if strings.TrimSpace(f.Data) == "" {
				return nil, fmt.Errorf("minimax file_data is empty, file=%s", f.FileName)
			}
			if mt := strings.ToLower(strings.TrimSpace(f.MediaType)); mt != "" && mt != "application/pdf" {
				return nil, fmt.Errorf("minimax non-text file currently only supports pdf, file=%s", f.FileName)
			}
			pdfDataURL, err := buildMiniMaxPDFDataURL(f.Data)
			if err != nil {
				return nil, fmt.Errorf("minimax pdf file_data invalid, file=%s: %w", f.FileName, err)
			}
			blocks = append(blocks, minimaxContentBlock{
				Type: "file",
				File: &minimaxFileSource{
					FileData: pdfDataURL,
					FileName: strings.TrimSpace(f.FileName),
				},
			})
		}
		out = append(out, minimaxFileMessage{
			Role:    msg.Role,
			Content: blocks,
		})
	}
	return out, nil
}

func minimaxFindLatestUserMessageIndex(messages []LLMMessage) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, "user") {
			return i
		}
	}
	return -1
}

func buildMiniMaxPDFDataURL(base64Data string) (string, error) {
	data := strings.TrimSpace(base64Data)
	if strings.HasPrefix(data, "data:") {
		lower := strings.ToLower(data)
		if !strings.HasPrefix(lower, "data:application/pdf;") {
			return "", fmt.Errorf("data url mime must be application/pdf")
		}
		return data, nil
	}
	return fmt.Sprintf("data:application/pdf;base64,%s", data), nil
}

func buildMiniMaxTextFileBlock(fileName, text string) string {
	name := strings.TrimSpace(fileName)
	content := strings.TrimSpace(text)
	if content == "" {
		return ""
	}
	if name == "" {
		return content
	}
	return fmt.Sprintf("【文件 %s 内容】\n%s", name, content)
}

func (m *MiniMaxClient) ChatStreamWithTools(
	ctx context.Context,
	messages []ChatMessage,
	model string,
	tools []ToolDefinition,
) (<-chan StreamChunk, error) {
	if model == "" {
		model = m.defaultModel
	}
	maxTokens := m.config.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	apiMessages := buildMiniMaxToolMessages(messages)

	reqBody := minimaxToolRequest{
		Model:               model,
		Messages:            apiMessages,
		MaxCompletionTokens: maxTokens,
		Stream:              true,
		StreamOptions:       &minimaxStreamOptions{IncludeUsage: true},
		Tools:               toMiniMaxTools(tools),
		ToolChoice:          toolChoiceFromContext(ctx),
	}

	bodyBytes, err := marshalRequestBodyGuard(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	logLLMPrefixDebug(ctx, "MiniMax", bodyBytes)

	apiURL := m.config.ApiUrl + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	applyTraceHeadersFromContext(ctx, req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("minimax api error (status %d): %s", resp.StatusCode, string(body))
	}

	ch := make(chan StreamChunk, 64)
	go m.parseMiniMaxToolStream(ctx, resp, ch)
	return ch, nil
}

func (m *MiniMaxClient) parseMiniMaxToolStream(ctx context.Context, resp *http.Response, ch chan<- StreamChunk) {
	defer close(ch)
	defer resp.Body.Close()

	var (
		inputTokens, outputTokens int
		cacheReadTokens           int
		cacheCreateTokens         int
		finishReason              string
		toolCallBuilders          = make(map[int]*minimaxToolCallBuilder)
	)

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 256*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ch <- StreamChunk{
				Done:              true,
				Cancelled:         true,
				InputTokens:       inputTokens,
				OutputTokens:      outputTokens,
				CacheReadTokens:   cacheReadTokens,
				CacheCreateTokens: cacheCreateTokens,
			}
			return
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk minimaxStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Usage != nil {
			inputTokens = chunk.Usage.PromptTokens
			outputTokens = chunk.Usage.CompletionTokens
			cacheReadTokens = chunk.Usage.PromptTokensDetails.CachedTokens
		}

		for _, choice := range chunk.Choices {
			if reasoning := firstNonEmpty(choice.Delta.ReasoningContent, choice.Delta.Reasoning); reasoning != "" {
				ch <- StreamChunk{ReasoningContent: reasoning}
			}
			if choice.Delta.Content != "" {
				ch <- StreamChunk{Content: choice.Delta.Content}
			}

			for _, tc := range choice.Delta.ToolCalls {
				b, ok := toolCallBuilders[tc.Index]
				if !ok {
					b = &minimaxToolCallBuilder{}
					toolCallBuilders[tc.Index] = b
				}
				if tc.ID != "" {
					b.id = tc.ID
				}
				if tc.Function != nil {
					if tc.Function.Name != "" {
						b.name = tc.Function.Name
					}
					b.argsBuf.WriteString(tc.Function.Arguments)
				}
			}

			if choice.FinishReason != nil {
				finishReason = *choice.FinishReason
			}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{
			Error:             fmt.Errorf("read stream: %w", err),
			Done:              true,
			InputTokens:       inputTokens,
			OutputTokens:      outputTokens,
			CacheReadTokens:   cacheReadTokens,
			CacheCreateTokens: cacheCreateTokens,
		}
		return
	}

	var stopReason string
	var toolCalls []ToolCall

	if hasMiniMaxToolCallBuilders(toolCallBuilders) || isToolCallFinishReason(finishReason) {
		stopReason = "tool_use"
		for i := 0; i < len(toolCallBuilders); i++ {
			if b, ok := toolCallBuilders[i]; ok {
				toolCalls = append(toolCalls, ToolCall{
					ID:    b.id,
					Name:  b.name,
					Input: canonicalToolInput(json.RawMessage(b.argsBuf.String())),
				})
			}
		}
	} else {
		stopReason = "end_turn"
	}

	ch <- StreamChunk{
		Done:              true,
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		CacheReadTokens:   cacheReadTokens,
		CacheCreateTokens: cacheCreateTokens,
		StopReason:        stopReason,
		ToolCalls:         toolCalls,
	}
}

type minimaxToolCallBuilder struct {
	id      string
	name    string
	argsBuf strings.Builder
}

func hasMiniMaxToolCallBuilders(builders map[int]*minimaxToolCallBuilder) bool {
	for _, b := range builders {
		if b != nil && (strings.TrimSpace(b.id) != "" || strings.TrimSpace(b.name) != "" || b.argsBuf.Len() > 0) {
			return true
		}
	}
	return false
}

func buildMiniMaxToolMessages(messages []ChatMessage) []minimaxAnyMsg {
	out := make([]minimaxAnyMsg, 0, len(messages))

	for _, msg := range messages {
		if len(msg.Parts) == 0 {
			out = append(out, minimaxAnyMsg{Role: msg.Role, Content: msg.Content})
			continue
		}

		switch msg.Role {
		case "assistant":
			am := minimaxAnyMsg{Role: "assistant"}
			var textParts []string
			for _, p := range msg.Parts {
				switch p.Type {
				case "text":
					textParts = append(textParts, p.Text)
				case "tool_use":
					am.ToolCalls = append(am.ToolCalls, minimaxToolCallMsg{
						ID:   p.ID,
						Type: "function",
						Function: minimaxFuncCall{
							Name:      p.Name,
							Arguments: string(canonicalToolInput(p.Input)),
						},
					})
				}
			}
			if len(textParts) > 0 {
				am.Content = strings.Join(textParts, "")
			}
			out = append(out, am)

		case "user":
			for _, p := range msg.Parts {
				if p.Type == "tool_result" {
					out = append(out, minimaxAnyMsg{
						Role:       "tool",
						Content:    p.Content,
						ToolCallID: p.ToolUseID,
					})
				}
			}

		default:
			out = append(out, minimaxAnyMsg{Role: msg.Role, Content: msg.Content})
		}
	}

	return out
}

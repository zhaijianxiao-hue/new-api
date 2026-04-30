package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestResponsesStreamKeepsToolCallAfterText(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldStreamingTimeout
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(common.RequestIdKey, "test-responses-tool-after-text")

	body := strings.Join([]string{
		`sse: ignored`,
		`data: {"type":"response.created","response":{"model":"gpt-5.5","created_at":1777536000}}`,
		`data: {"type":"response.output_text.delta","delta":"我先检查一下。"}`,
		`data: {"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"shell_command","status":"in_progress"}}`,
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"command\":\"git status\"}"}`,
		`data: {"type":"response.output_item.done","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"shell_command","status":"completed","arguments":"{\"command\":\"git status\"}"}}`,
		`data: {"type":"response.completed","response":{"status":"completed","model":"gpt-5.5","created_at":1777536000,"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}}}`,
		``,
	}, "\n")
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	info := &relaycommon.RelayInfo{
		StartTime:   time.Now(),
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.5",
		},
	}

	_, err := OaiResponsesToChatStreamHandler(c, info, resp)
	if err != nil {
		t.Fatalf("OaiResponsesToChatStreamHandler returned error: %v", err)
	}

	out := recorder.Body.String()
	if !strings.Contains(out, `"tool_calls"`) {
		t.Fatalf("expected streamed tool call after text, got: %s", out)
	}
	if !strings.Contains(out, `"finish_reason":"tool_calls"`) {
		t.Fatalf("expected final finish_reason tool_calls, got: %s", out)
	}
}

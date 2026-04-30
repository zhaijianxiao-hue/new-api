package service

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestClaudeToOpenAIRequestMapsCodexOutputEffort(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeCodex,
		},
	}
	claudeRequest := dto.ClaudeRequest{
		Model:        "gpt-5.5",
		OutputConfig: json.RawMessage(`{"effort":"high"}`),
	}

	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, info)
	if err != nil {
		t.Fatalf("ClaudeToOpenAIRequest returned error: %v", err)
	}

	if openAIRequest.ReasoningEffort != "high" {
		t.Fatalf("ReasoningEffort = %q, want high", openAIRequest.ReasoningEffort)
	}
	if info.RequestDebug["claude_output_effort"] != "high" {
		t.Fatalf("expected request debug to record claude_output_effort, got %+v", info.RequestDebug)
	}
	if info.RequestDebug["openai_reasoning_effort"] != "high" {
		t.Fatalf("expected request debug to record openai_reasoning_effort, got %+v", info.RequestDebug)
	}
}

func TestStreamResponseOpenAI2ClaudeUsesCachedUsageForFinalStop(t *testing.T) {
	finishReason := "stop"
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeText,
			Usage: &dto.Usage{
				PromptTokens:     7,
				CompletionTokens: 3,
				TotalTokens:      10,
			},
		},
	}
	openAIResponse := &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				FinishReason: &finishReason,
				Index:        0,
			},
		},
	}

	responses := StreamResponseOpenAI2Claude(openAIResponse, info)

	assertClaudeEventTypes(t, responses, []string{
		"content_block_stop",
		"message_delta",
		"message_stop",
	})
	if !info.ClaudeConvertInfo.Done {
		t.Fatal("expected Claude conversion to be marked done")
	}
	if responses[1].Usage == nil {
		t.Fatal("expected final message_delta to include cached usage")
	}
	if responses[1].Usage.InputTokens != 7 || responses[1].Usage.OutputTokens != 3 {
		t.Fatalf("unexpected usage: %+v", responses[1].Usage)
	}
}

func assertClaudeEventTypes(t *testing.T, responses []*dto.ClaudeResponse, want []string) {
	t.Helper()
	if len(responses) != len(want) {
		t.Fatalf("expected %d responses, got %d: %+v", len(want), len(responses), responses)
	}
	for i, response := range responses {
		if response.Type != want[i] {
			t.Fatalf("response %d type = %q, want %q", i, response.Type, want[i])
		}
	}
}

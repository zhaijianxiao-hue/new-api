package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestForceResponsesStreamForCodexChannel(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeCodex}}
	req := &dto.OpenAIResponsesRequest{}

	forceResponsesStreamIfRequired(info, req)

	require.NotNil(t, req.Stream)
	require.True(t, *req.Stream)
	require.True(t, info.IsStream)
}

func TestForceResponsesStreamOverridesFalseForCodexChannel(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeCodex}}
	req := &dto.OpenAIResponsesRequest{Stream: common.GetPointer(false)}

	forceResponsesStreamIfRequired(info, req)

	require.NotNil(t, req.Stream)
	require.True(t, *req.Stream)
	require.True(t, info.IsStream)
}

func TestForceResponsesStreamDoesNotChangeOtherChannels(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI}}
	req := &dto.OpenAIResponsesRequest{Stream: common.GetPointer(false)}

	forceResponsesStreamIfRequired(info, req)

	require.NotNil(t, req.Stream)
	require.False(t, *req.Stream)
	require.False(t, info.IsStream)
}

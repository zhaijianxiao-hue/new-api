# LEARNINGS

### 1. Codex OAuth Upstream Requires Streaming

**Problem**: Claude Code and other agent clients can fail against the Codex
channel with `Stream must be set to true`. Cherry Studio can also fail with
`Invalid URL (POST /v1/responses/chat/completions)` when its base URL includes
`/responses`.

**Root cause**: Codex only supports the Responses upstream flow and requires
streaming. Some clients omit `stream` or set it false. A channel-level
`param_override` only changes the JSON body and does not update relay internal
state, so SSE may be parsed as non-stream JSON.

**Solution**: Keep Codex routing through Chat/Claude to Responses compatibility,
and force both `request.Stream = true` and `info.IsStream = true` for
`constant.ChannelTypeCodex` in `relay/chat_completions_via_responses.go`.

**Prevention rule**: Before debugging Codex/OAuth/Claude Code agent requests,
read `docs/codex-stream-compat.md`. Do not solve this by only adding
`{"stream":true}` to channel `param_override`.

### 2. Claude Code Requires Complete SSE Closing Events

**Problem**: Claude Code can report `API returned an empty or malformed response
(HTTP 200)` even when new-api logs the request as successful.

**Root cause**: The Claude-compatible stream must end with
`content_block_stop`, `message_delta`, and `message_stop`. When new-api converts
Codex Responses streaming back through Chat Completions into Claude format, the
final synthesized OpenAI stop chunk may not contain inline `usage`. The usage is
cached in `info.ClaudeConvertInfo.Usage`; the converter must use that cached
usage instead of returning no events.

**Solution**: In `service/convert.go`, `StreamResponseOpenAI2Claude` should only
defer a finish chunk when neither inline usage nor cached Claude conversion usage
exists. The regression test is
`service/convert_claude_stream_test.go`.

**Prevention rule**: Verify Claude-compatible streams include all closing SSE
events, not just `message_start` and `content_block_delta`.

### 3. Do Not Drop Tool Calls After Assistant Text

**Problem**: Claude Code can stop after one short sentence such as "I will check
the current changes..." instead of actually running tools.

**Root cause**: Codex Responses streaming may emit assistant text before
`function_call` output items. The previous compatibility handler ignored tool
calls once any output text had been streamed, so Claude Code received only the
short preface and a normal end-turn.

**Solution**: In `relay/channel/openai/chat_via_responses.go`, always preserve
Responses `function_call` items even if text was already streamed, and use
Chat Completions `finish_reason: tool_calls` whenever a tool call was seen.
The regression test is
`relay/channel/openai/chat_via_responses_test.go`.

**Prevention rule**: For Codex OAuth + Claude Code, text-before-tool is valid.
Never assume text output means tool calls should be discarded.

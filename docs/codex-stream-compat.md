# Codex OAuth Stream Compatibility

This repository is deployed locally as a Docker-based new-api gateway with a
Codex (OpenAI OAuth) channel. Read this note before changing, pulling,
rebuilding, or debugging Codex/Claude Code/agent calls.

## Symptom

Clients that call the Codex channel through OpenAI-compatible or
Claude-compatible endpoints may fail with:

```text
Stream must be set to true
```

Claude Code can also fail after receiving HTTP 200:

```text
API returned an empty or malformed response (HTTP 200)
```

Observed failing route:

```text
POST /v1/messages?beta=true
```

Observed earlier endpoint configuration mistake:

```text
POST /v1/responses/chat/completions
```

`/v1/responses/chat/completions` is invalid. Valid new-api routes are:

```text
POST /v1/chat/completions
POST /v1/responses
POST /v1/messages?beta=true
```

For Cherry Studio or OpenAI-compatible clients, use this base URL:

```text
http://localhost:9847/v1
```

Do not use:

```text
http://localhost:9847/v1/responses
```

## Root Cause

The Codex channel only supports the upstream Responses flow. Its adapter rejects
plain Chat Completions and Claude Messages unless they are converted through the
Responses path.

Some clients, including Claude Code and other agents, may omit `stream` or send
`stream: false` on internal requests. The Codex upstream requires streaming and
returns `Stream must be set to true` when the upstream Responses request is not
streaming.

It is not enough to set channel `param_override` to:

```json
{"stream": true}
```

That changes the outgoing request body, but the relay's internal `info.IsStream`
state can still be false. Then new-api may treat the SSE upstream response as a
normal JSON body and fail with an error like:

```text
invalid character 'e' looking for beginning of value
```

Even after the stream flag is correct, Claude-compatible clients require the
SSE stream to finish with Anthropic-style closing events. If the OpenAI
Responses upstream reaches EOF and new-api synthesizes the final Chat
Completions stop chunk, the Claude converter must still emit:

```text
event: content_block_stop
event: message_delta
event: message_stop
```

If these are missing, Claude Code can report HTTP 200 as malformed because the
stream shape is incomplete.

Codex Responses streaming can also emit normal assistant text before tool call
items in the same response. Claude Code commonly does this for actions like
`git status`: the model first says "I will check..." and then emits
`function_call` output items. The gateway must keep those tool calls even when
text has already been streamed, and the final Chat Completions finish reason
must be `tool_calls` so the Claude converter emits `stop_reason: tool_use`.

## Required Code Behavior

When `chatCompletionsViaResponses` is used for the Codex channel
(`constant.ChannelTypeCodex`), the relay must force both:

```go
request.Stream = common.GetPointer(true)
info.IsStream = true
```

The fix lives in:

```text
relay/chat_completions_via_responses.go
```

The regression test lives in:

```text
relay/chat_completions_via_responses_test.go
```

For Claude-compatible output, `StreamResponseOpenAI2Claude` must use cached
usage from `info.ClaudeConvertInfo.Usage` when the final OpenAI stop chunk has
`finish_reason` but no inline `usage`. This allows it to emit the required
Claude stream closing events instead of returning an empty event list.

That fix lives in:

```text
service/convert.go
```

The regression test lives in:

```text
service/convert_claude_stream_test.go
```

When converting Responses streaming output back to Chat Completions, do not drop
`function_call` items merely because `response.output_text.delta` was already
seen. The fix lives in:

```text
relay/channel/openai/chat_via_responses.go
```

The regression test lives in:

```text
relay/channel/openai/chat_via_responses_test.go
```

Run the focused test after touching this behavior:

```powershell
go test ./relay -run TestForceResponsesStream -count=1
go test ./service -run TestStreamResponseOpenAI2ClaudeUsesCachedUsageForFinalStop -count=1
go test ./relay/channel/openai -run TestResponsesStreamKeepsToolCallAfterText -count=1
```

## Required Runtime Settings

The Codex channel should not rely on `param_override` for this stream fix.
Verify it is empty/null:

```powershell
docker exec postgres psql -U root -d new-api -c "SELECT id, name, param_override FROM channels WHERE type = 57;"
```

The Chat Completions to Responses compatibility policy must be enabled for the
Codex channel type:

```sql
INSERT INTO options (key, value)
VALUES (
  'global.chat_completions_to_responses_policy',
  '{"enabled":true,"all_channels":false,"channel_types":[57],"model_patterns":["^gpt-5.*$"]}'
)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
```

After changing this option, restart new-api or wait for option sync:

```powershell
docker restart new-api
```

## Rebuild Notes

If you pull new code or rebuild the Docker container, make sure the code fix is
present before starting the service. If the official image is pulled again
without this patch, the bug can return.

Typical local rebuild path:

```powershell
cd D:\Workbench\github\new-api

cd web\default
bun install
$env:DISABLE_ESLINT_PLUGIN='true'
$env:VITE_REACT_APP_VERSION=(Get-Content ..\..\VERSION)
bun run build

cd ..\classic
bun install
$env:VITE_REACT_APP_VERSION=(Get-Content ..\..\VERSION)
bun run build

cd ..\..
$env:GOOS='linux'
$env:GOARCH='amd64'
$env:CGO_ENABLED='0'
$env:GOEXPERIMENT='greenteagc'
go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(Get-Content .\VERSION)'" -o .\bin\new-api-patched .
```

To patch an already-running container without rebuilding the image:

```powershell
docker cp .\bin\new-api-patched new-api:/new-api.patched
docker exec new-api sh -c 'chmod +x /new-api.patched && mv /new-api.patched /new-api'
docker restart new-api
```

## Verification

1. Verify unit behavior:

```powershell
go test ./relay -run TestForceResponsesStream -count=1
```

2. Verify a Claude-compatible call without an explicit `stream` can still return
   a Claude SSE stream:

```powershell
$token = docker exec postgres psql -U root -d new-api -t -A -c "SELECT key FROM tokens WHERE name = 'claude-code' AND deleted_at IS NULL LIMIT 1;"
$body = @{model='gpt-5.4'; max_tokens=64; messages=@(@{role='user'; content='hello'})} | ConvertTo-Json -Depth 10 -Compress
$tmp = Join-Path $env:TEMP 'new-api-claude-message-body.json'
Set-Content -LiteralPath $tmp -Value $body -NoNewline
curl.exe -sS --max-time 25 -N `
  -H "Authorization: Bearer $token" `
  -H "Content-Type: application/json" `
  -H "anthropic-version: 2023-06-01" `
  --data-binary "@$tmp" `
  "http://localhost:9847/v1/messages?beta=true"
```

Expected response begins with SSE events like:

```text
event: message_start
event: content_block_start
event: content_block_delta
event: content_block_stop
event: message_delta
event: message_stop
```

There should be no upstream error containing:

```text
Stream must be set to true
```

There should also be no Claude Code error saying HTTP 200 was empty or
malformed.

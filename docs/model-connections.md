# Runtime and model connections

Runtime implementation and provider authentication are independent axes:

| Runtime          | ChatGPT OAuth | Direct HTTP API key                                                         | Notes                                                              |
| ---------------- | ------------- | --------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| Pi SDK           | Yes           | Anthropic Messages, OpenAI Responses, or OpenAI-compatible Chat Completions | Pi owns the full agent session                                     |
| Built-in Runtime | Yes           | The same three protocols                                                    | Application-owned lightweight loop; model-only provider transport  |
| Codex App Server | Yes           | OpenAI Responses only                                                       | Private native session and loop; no application-server replacement |
| Claude Agent SDK | No            | Anthropic Messages                                                          | Official SDK subprocess with only host advertising tools           |

Direct connections are explicit; the application never guesses a wire protocol from a URL.
An environment-variable **name**, not its key value, is stored in session configuration.
Alternatively, the authenticated Web Settings form accepts a key for the current server
process. It is sent once to the server and is not returned by the settings API or persisted
in SQLite, logs or Git. Credentials never enter model business context.

Example OpenAI-compatible provider through Pi (or `--runtime builtin`):

```sh
export DEEPSEEK_API_KEY='set locally; never commit'
./bin/ad-agent chat --runtime pi --session deepseek-lab \
  --model-auth api_key --provider deepseek --model deepseek-chat \
  --model-api openai-completions --model-base-url https://api.deepseek.com \
  --model-api-key-env DEEPSEEK_API_KEY --model-context-window 128000 \
  --model-max-output-tokens 8192 --message 'Read the account and list its campaigns.'
```

Example Claude Agent SDK runtime:

```sh
export ANTHROPIC_API_KEY='set locally; never commit'
./bin/ad-agent chat --runtime claude --session claude-lab \
  --model-auth api_key --provider anthropic --model YOUR_CLAUDE_MODEL_ID \
  --model-api anthropic-messages --model-base-url https://api.anthropic.com \
  --model-api-key-env ANTHROPIC_API_KEY --model-context-window 200000 \
  --model-max-output-tokens 8192 --message 'Read the account and list its campaigns.'
```

Web Settings separates runtime selection from model connections. Choose ChatGPT OAuth,
OpenRouter OAuth, or direct HTTP with an explicit provider, protocol, URL and model. Direct
HTTP accepts an environment-variable reference or a server-memory key. OpenRouter OAuth
uses PKCE to obtain a server-memory key and then uses the HTTP provider transport; it is
not a ChatGPT subscription connection. Memory-only credentials must be supplied again
after a server restart.

Built-in Runtime, Pi and Codex can use both connection modes, but Codex requires Responses for direct HTTP.
OpenRouter's default Chat Completions connection therefore requires Built-in Runtime or Pi, not Codex.
Claude SDK requires the Anthropic API-key
configuration above. Explicit runtime or model changes keep the current conversation;
the next turn discards an incompatible native checkpoint and rebuilds from saved public
context. The selected provider receives that context, not another provider's private
reasoning or credentials. An ad-account/environment change still isolates the conversation.
These are experimental workspace settings, not
a public plugin protocol; see [architecture boundaries](architecture.md#independent-replacement-boundaries).

Codex reuses the existing Pi ChatGPT credential resolver, including refresh, without
using Pi's agent loop or changing global defaults. Its native process stores credentials
ephemerally. Direct connections pass only the chosen environment-variable credential.
The native engine owns its output-token policy: the shared declared max-output value is
not a Codex request-token cap. Context-window configuration is forwarded; the application
still enforces response-size, tool-count and deadline limits. See [Codex Runtime](codex-runtime.md).

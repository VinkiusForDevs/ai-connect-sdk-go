# Vinkius Connect for Go

### Build AI agents that act for each user.

One SDK for user-specific tools, credentials, permissions, and execution across 11,000+ systems. This Go SDK has zero runtime dependencies: it is built on the standard library.

[![Connectors](https://api.vinkius.com/badges/connectors.svg)](https://vinkius.com)
[![AI Capabilities](https://api.vinkius.com/badges/capabilities.svg)](https://vinkius.com)

[Documentation](https://vinkius.com/learn/en/connect-sdk) · [Vinkius Cloud](https://cloud.vinkius.com) · [Site](https://vinkius.com)

---

AI applications become more useful when they can take action in the systems their users already work with. But shipping those actions means dealing with integrations, OAuth, credential storage, permissions, API changes, and model-specific tool formats.

**Vinkius Connect handles that connectivity layer.** Your application keeps its users and customer experience. Vinkius resolves each user's connected systems and available capabilities, executes actions on their behalf, and returns the result.

```text
Your AI application
        │
        │  Your authenticated user's external_id
        ▼
Vinkius Connect
  identity · connections · credentials · capabilities · execution
        │
        ├── GitHub
        ├── Slack
        ├── Project management
        └── 11,000+ connected systems
```

## Why Vinkius Connect?

- **Built for multi-user AI products.** Each user has their own connections and available capabilities.
- **One consistent interface.** Discover and execute actions without implementing each integration yourself.
- **Credentials stay out of model context.** Connector secrets are write-only through the documented API and are not returned to application code.
- **Framework flexibility.** Convert capabilities to the tool formats of OpenAI, Anthropic, Google Gemini, or a neutral JSON-Schema shape.
- **Managed connectivity.** Vinkius maintains the connectors and their evolving APIs.
- **Your product stays yours.** Use your application's own user IDs and keep the customer relationship and user experience in your product.

## How it works

1. **Identify the user.** Pass the stable `external_id` assigned by your application backend.
2. **Discover capabilities.** Vinkius resolves the user's ready connectors and the actions they expose.
3. **Provide tools to the model.** Convert those capabilities into the tool format your model expects.
4. **Execute the selected action.** Send the tool call to Vinkius and return the result to your agent loop.

The `external_id` is an identifier, **not an authentication mechanism**. Only pass an ID that your backend has associated with the currently authenticated user. Never trust a user ID supplied directly by an untrusted client.

## Installation

Go **1.21+** is required. The module is published to GitHub and fetched by its tag:

```bash
go get github.com/VinkiusForDevs/ai-connect-sdk-go@latest
```

## Quick start

Create an AI application in [Vinkius Cloud](https://cloud.vinkius.com), then create an application key. Keep the key on your server. **Never expose it in browser code, mobile clients, or model prompts.**

```go
package main

import (
	"context"
	"fmt"
	"os"

	vk "github.com/VinkiusForDevs/ai-connect-sdk-go/vinkius"
)

func main() {
	// Use the ID of the authenticated user from your own backend.
	v, err := vk.New(os.Getenv("VINKIUS_APP_ID"), os.Getenv("VINKIUS_APP_KEY"))
	if err != nil {
		panic(err)
	}
	defer v.Close()

	user, err := v.User("alice_123")
	if err != nil {
		panic(err)
	}

	// Discover actions available through this user's ready connectors.
	set, err := user.Capabilities(context.Background(), nil)
	if err != nil {
		panic(err)
	}

	// Execute one capability by its namespaced name.
	capability := set.FindCapability("github__create_issue")
	result, err := capability.Execute(context.Background(), map[string]any{
		"owner": "acme",
		"repo":  "product",
		"title": "Ship it",
	}, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Text())
}
```

The capability name and input schema depend on the connector and capabilities available to that user. Discover the available capabilities before attempting to execute one.

## Use capabilities with a model

The usual agent loop is straightforward:

1. Fetch the authenticated user's capabilities.
2. Convert them into the model's tool format.
3. Send the user's request and tools to the model.
4. Execute the returned tool call through Vinkius.
5. Return the result to the model as part of your agent loop.

```go
caps, _ := user.Capabilities(ctx, nil)

// OpenAI Chat Completions tools shape.
tools, err := adapters.ToOpenAITools(caps)

// After the model returns a tool call:
result, err := adapters.RunOpenAIToolCall(ctx, caps, adapters.OpenAIToolCall{
	Function: adapters.OpenAIFunction{
		Name:      "github__create_issue",
		Arguments: `{"owner":"acme","repo":"product","title":"Ship it"}`,
	},
})
fmt.Println(result.Text())
```

Production agent loops should handle each tool call and send its result back to the model using the provider's expected format.

## Connect a user's system

Provision a connector for a user, set its credentials, check its status, and inspect its capabilities:

```go
github := user.Connector("github")

conn, _ := github.Connect(ctx, nil) // Get or create the connection
fmt.Println(conn["runtime_url"])

_ = github.Credentials.Set(ctx, map[string]any{"GITHUB_TOKEN": token}, nil)

status, _ := github.Status(ctx, nil) // not_connected, needs_credentials, ready, or disabled

capabilities, _ := github.Capabilities(ctx, nil)
```

The example assumes `v` is already configured and `token` was obtained securely. In a production application, prefer the supported authorization flow for the connector where available, and never ask users to paste secrets into an AI conversation.

Credential values are write-only through the documented interface. The credential schema and whether a value is set can be inspected; secret values cannot be read back.

## Framework adapters

A capability set can be converted to different tool formats. Adapter functions shape the tools; they do not require the target framework merely to perform the conversion.

| Framework / format | Function | Run |
| --- | --- | --- |
| OpenAI Chat Completions | `adapters.ToOpenAITools` | `adapters.RunOpenAIToolCall` |
| Anthropic Messages | `adapters.ToAnthropicTools` | `adapters.RunAnthropicToolUse` |
| Google Gemini | `adapters.ToGeminiTools` | `adapters.RunGeminiFunctionCall` |
| Framework-neutral JSON Schema | `adapters.ToJSONSchemaTools` | `adapters.ExecuteByName` |

```go
openaiTools, _  := adapters.ToOpenAITools(caps)
anthropicTools, _ := adapters.ToAnthropicTools(caps)
geminiTools, _  := adapters.ToGeminiTools(caps)
jsonTools       := adapters.ToJSONSchemaTools(caps)
```

## Core concepts

| Concept | Meaning |
| --- | --- |
| **Connector** | An external system a user connects, such as GitHub or Slack. |
| **Capability** | An executable action exposed by a connector, such as listing repositories or creating an issue. |
| **User** | A user identified by your application's `external_id`. Vinkius does not require a separate end-user identity. |
| **Control plane** | Provisioning and state management for users, connectors, credentials, and catalog discovery. |
| **Execution plane** | Runtime capability discovery and execution, per user through a short-lived token. |

### Capability discovery behavior

`user.Capabilities` resolves the user's ready connectors in parallel, with concurrency capped at eight. A single unresponsive connector does not necessarily fail the whole batch. Use `OnConnectorError` to observe individual connector failures. If all connectors fail, the first error is raised.

## Configuration

```go
v, _ := vk.New(
	appID,
	apiKey,
	vk.WithTimeout(30*time.Second),  // default
	vk.WithMaxRetries(2),           // default; retry-safe requests only
	vk.WithDoer(nil),               // optionally inject a core.Doer
	vk.WithUserAgent("acme-ai/1.0"),
	vk.WithNamespaceCapability(func(connector, name string) string {
		return connector + "__" + name
	}),
	vk.WithHooks(myHooks),
)
```

- **`appID`** identifies your Vinkius application.
- **`apiKey`** is a server-side secret. Keep it out of clients and prompts.
- **`WithTimeout`** sets the per-request deadline.
- **`WithMaxRetries`** controls retries for retry-safe requests.
- **`WithDoer`** lets you inject a `core.Doer` transport, useful for tests and custom transports.
- **`WithUserAgent`** appends your application's identifier to the SDK user agent.
- **`WithNamespaceCapability`** controls how connector and capability names are combined.
- **`WithHooks`** configures observability hooks; sensitive headers and bodies are redacted before hooks run.

Call `v.Close()` to release the SDK's HTTP client.

### Retries and idempotency

Transient HTTP statuses (`429`, `502`, `503`, `504`) and network errors are retried with full-jitter backoff only for retry-safe requests. Idempotent methods and requests carrying an `IdempotencyKey` may be retried. Non-idempotent POST requests, such as minting a connection token, are not automatically retried.

For actions that create or modify data in external systems, design your agent loop to account for duplicate requests and uncertain outcomes. Use idempotency support where available and appropriate.

## Low-level API

The fluent API has REST equivalents when you need more direct control:

```go
connection, _ := v.Users.Connections("alice_123")
created, _ := connection.Create(ctx, map[string]any{"connector": "github"}, nil)

creds, _ := v.Users.Connections("alice_123")
_, _ = creds.Credentials(created["id"].(string)).Set(ctx, map[string]any{
	"credentials": map[string]any{"GITHUB_TOKEN": "..."},
}, nil)

search, _ := v.Catalog.Search(ctx, "github", nil)

for _, connector := range search {
	_ = connector
}
```

## Error handling

Failures surface as a single `*vk.Error` (an alias of `core.Error`) carrying a status, a stable error code, and the server `request_id` where available. Use the exported predicates to branch without type assertion:

- `vk.IsAuthError`, `vk.IsNotFoundError`, `vk.IsValidationError`
- `vk.IsRateLimitError`, `vk.IsQuotaError`, `vk.IsOverageError`
- `vk.IsConnectorNotConnectedError`, `vk.IsConnectionError`, `vk.IsProtocolError`

```go
set, err := user.Capabilities(ctx, nil)
if err != nil {
	if vk.IsRateLimitError(err) {
		// Inspect RetryAfterMS when present.
		_ = vk.SDKError(err).RetryAfterMS
	} else {
		e := vk.SDKError(err)
		fmt.Println(e.Code, e.Status, e.RequestID)
	}
}
```

Capability-level execution failures are returned as results with `IsError` set, allowing an agent loop to pass the failure back to the model and recover. Transport, configuration, and protocol failures are returned as errors.

## Security practices

- **Keep application keys server-side.** Never ship `vk_app_sk_*` in frontend code, mobile applications, repositories, or prompts.
- **Authenticate users in your application.** Resolve `external_id` from trusted backend state; do not accept arbitrary user IDs from clients.
- **Scope actions to the right user.** Fetch and execute capabilities for the authenticated user's identity only.
- **Keep raw secrets out of prompts.** Connector credentials are write-only through the documented API and are not intended for model context.
- **Treat tool calls as actions.** Apply your product's confirmation, authorization, and policy requirements before executing sensitive or irreversible operations.
- **Protect logs.** Do not log API keys, tokens, credential values, or sensitive customer data.
- **Handle revocation and failures.** Make connection state visible in your product and define how your application responds when a connector is disabled, expired, or unavailable.

Vinkius Connect provides connectivity and execution primitives. Your application remains responsible for authenticating its users, enforcing product-level policy, and deciding when an AI-requested action requires confirmation.

## Development

Clone the repository and run the tests. The SDK's tests use an injectable fake transport and run offline:

```bash
go test ./...
go vet ./...
```

## License

[Apache License 2.0](./LICENSE), Copyright Vinkius.

package vinkius

import (
	"runtime"
	"strings"
	"time"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
	"github.com/VinkiusForDevs/ai-connect-sdk-go/resources"
)

// Vinkius is the SDK root, scoped to a single application. It holds the only
// secret the integrator manages (vk_app_sk_*) and the public app id (vk_app_*).
// Configuration is validated eagerly and fails fast with a config error.
type Vinkius struct {
	ctx *sdkContext

	// Catalog is public-connector discovery.
	Catalog *resources.CatalogClient
	// Users is the low-level users client, scoped to this application.
	Users *resources.AppUsersClient

	doer     core.Doer
	ownsDoer bool
}

// defaultBaseURL is the immutable Vinkius control-plane endpoint. It is never
// user-settable and is not surfaced in configuration.
const defaultBaseURL = "https://api.vinkius.com"

const defaultTimeout = 30 * time.Second

// config is the resolved configuration behind the functional options.
type config struct {
	appID          string
	apiKey         string
	timeout        time.Duration
	maxRetries     int
	haveMaxRetries bool
	doer           core.Doer
	userAgent      string
	hooks          core.Hooks
	namespace      func(connector, name string) string
}

// Option configures the Vinkius client.
type Option func(*config)

// WithTimeout sets the per-request deadline that overrides nothing else.
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// WithMaxRetries sets the maximum automatic retries for idempotent requests.
func WithMaxRetries(n int) Option {
	return func(c *config) { c.maxRetries = n; c.haveMaxRetries = true }
}

// WithDoer injects a custom transport. The SDK does not own or close an
// injected doer; when omitted the SDK builds and owns its own.
func WithDoer(d core.Doer) Option {
	return func(c *config) { c.doer = d }
}

// WithUserAgent appends an extra token to the default User-Agent.
func WithUserAgent(s string) Option {
	return func(c *config) { c.userAgent = s }
}

// WithHooks installs observability hooks; callbacks receive only redacted data.
func WithHooks(h core.Hooks) Option {
	return func(c *config) { c.hooks = h }
}

// WithNamespaceCapability controls how a capability's display name is namespaced
// across connectors. The default is connector__name.
func WithNamespaceCapability(f func(connector, name string) string) Option {
	return func(c *config) { c.namespace = f }
}

// New builds a Vinkius client. appID must start with vk_app_ (and must not be
// the secret key); apiKey must start with vk_app_sk_.
func New(appID, apiKey string, opts ...Option) (*Vinkius, error) {
	cfg := config{appID: appID, apiKey: apiKey, timeout: defaultTimeout}
	for _, o := range opts {
		o(&cfg)
	}

	if !strings.HasPrefix(cfg.appID, "vk_app_") || strings.HasPrefix(cfg.appID, "vk_app_sk_") {
		return nil, core.NewConfigError(
			"`appId` must be a public application id starting with \"vk_app_\" (not the secret key).")
	}
	if !strings.HasPrefix(cfg.apiKey, "vk_app_sk_") {
		return nil, core.NewConfigError(
			"`apiKey` must be a secret application key starting with \"vk_app_sk_\".")
	}

	retry := core.RetryPolicy{MaxRetries: 2, BaseDelayMS: 250, MaxDelayMS: 4000}
	if cfg.haveMaxRetries {
		retry.MaxRetries = cfg.maxRetries
	}

	userAgent := "vinkius-connect/" + Version + " go/" + runtime.Version()
	if cfg.userAgent != "" {
		userAgent += " " + cfg.userAgent
	}

	doer := cfg.doer
	ownsDoer := false
	if doer == nil {
		doer = core.NewHTTPDoer()
		ownsDoer = true
	}

	http := core.NewHttpClient(core.HttpClientConfig{
		BaseURL:   defaultBaseURL,
		APIKey:    cfg.apiKey,
		AppID:     cfg.appID,
		Timeout:   cfg.timeout,
		Retry:     retry,
		Doer:      doer,
		UserAgent: userAgent,
		Hooks:     cfg.hooks,
	})

	namespace := cfg.namespace
	if namespace == nil {
		namespace = func(connector, name string) string { return connector + "__" + name }
	}

	ctx := &sdkContext{
		http:    http,
		appID:   cfg.appID,
		cache:   core.NewResolverCache[map[string]any](0),
		namespace: namespace,
		// The runtime client reuses the transport settings of the API client but
		// adds no API auth: the mcp_url embeds the vk_live_* token.
		runtime: func(mcpURL string) *core.RuntimeClient {
			return core.NewRuntimeClient(mcpURL, core.RuntimeClientConfig{
				Timeout:   cfg.timeout,
				Retry:     retry,
				Doer:      doer,
				UserAgent: userAgent,
				Hooks:     cfg.hooks,
			})
		},
	}

	return &Vinkius{
		ctx:      ctx,
		Catalog:  resources.NewCatalogClient(http),
		Users:    resources.NewAppUsersClient(http, cfg.appID),
		doer:     doer,
		ownsDoer: ownsDoer,
	}, nil
}

// User returns a lazy, zero-request handle to one end-user's capabilities.
func (v *Vinkius) User(externalID string) (*UserContext, error) {
	if externalID == "" {
		return nil, core.NewConfigError("`externalId` must be a non-empty string.")
	}
	if strings.HasPrefix(externalID, "vk_app_user_") {
		return nil, core.NewConfigError(
			"`externalId` must be YOUR user id (e.g. \"alice_123\"), not a Vinkius internal id. " +
				"The SDK resolves identifiers automatically from the external_id you provide.")
	}
	if err := core.AssertExternalID(externalID); err != nil {
		return nil, err
	}
	return &UserContext{ctx: v.ctx, externalID: externalID}, nil
}

// Close releases resources the SDK owns. It is a no-op for an injected doer the
// caller manages itself.
func (v *Vinkius) Close() {
	if !v.ownsDoer {
		return
	}
	if hd, ok := v.doer.(*core.HTTPDoer); ok {
		hd.Close()
	}
}

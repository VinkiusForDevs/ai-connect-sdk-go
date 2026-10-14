// Package vinkius is the Go SDK for Vinkius AI connectivity.
//
// Ship AI features, not integration projects. Give each user of your AI app
// their own connectors and AI capabilities.
//
// Domain vocabulary: users have connectors; a connected connector is a
// connection; the executable units an agent calls are capabilities.
//
// # Quick start
//
//	client, err := vinkius.New("vk_app_...", "vk_app_sk_...")
//	if err != nil {
//		return err
//	}
//	defer client.Close()
//
//	set, err := client.User("alice_123").Capabilities(context.Background(), nil)
//	if err != nil {
//		return err
//	}
//	for _, capability := range set.Items() {
//		_ = capability
//	}
//
// Use any goroutine-safe runtime (a plain goroutine, a FastAPI-style handler, an
// agent loop, etc.).
//
// # Errors
//
// Every failure surfaces as a *vinkius.Error carrying a stable Code and, when
// available, the server request id. Use the Is*Error predicates to branch on the
// typed error.
package vinkius

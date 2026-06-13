package limbgo

import "testing"

func TestProxyProtocolRequiredImpliesEnabled(t *testing.T) {
	runtime, err := newProxyProtocolRuntime(ProxyProtocolConfig{Required: true})
	if err != nil {
		t.Fatalf("proxy protocol runtime: %v", err)
	}
	if !runtime.enabled || !runtime.required {
		t.Fatalf("runtime = %+v, want enabled and required", runtime)
	}
}

func TestProxyProtocolDisabledIgnoresTrustedProxyParsing(t *testing.T) {
	if _, err := newProxyProtocolRuntime(ProxyProtocolConfig{TrustedProxies: []string{"not-an-ip"}}); err != nil {
		t.Fatalf("disabled proxy protocol parsed trusted proxies: %v", err)
	}
}

package main

import "testing"

func TestPolicyWants(t *testing.T) {
	cases := []struct {
		policy    string
		connected bool
		appActive bool
		want      bool
	}{
		{overlayPolicyAuto, false, false, false},
		{overlayPolicyAuto, true, true, false},
		{overlayPolicyAuto, true, false, true},
		{overlayPolicyAlways, false, false, false},
		{overlayPolicyAlways, true, true, true},
		{overlayPolicyAlways, true, false, true},
		{overlayPolicyManual, true, false, false},
		{overlayPolicyManual, true, true, false},
	}
	for _, c := range cases {
		if got := policyWants(c.policy, c.connected, c.appActive); got != c.want {
			t.Errorf("policyWants(%q, connected=%v, appActive=%v) = %v, want %v",
				c.policy, c.connected, c.appActive, got, c.want)
		}
	}
}

func TestToggleVisibilityOverridesAlwaysPolicy(t *testing.T) {
	overlayCfg.mu.Lock()
	prevPolicy := overlayCfg.policy
	prevVisible := overlayCfg.visible
	prevOverride := overlayCfg.manualOverride
	prevEnabled := overlayCfg.enabled
	overlayCfg.policy = overlayPolicyAlways
	overlayCfg.visible = true
	overlayCfg.manualOverride = nil
	overlayCfg.enabled = false
	overlayCfg.mu.Unlock()
	defer func() {
		overlayCfg.mu.Lock()
		overlayCfg.policy = prevPolicy
		overlayCfg.visible = prevVisible
		overlayCfg.manualOverride = prevOverride
		overlayCfg.enabled = prevEnabled
		overlayCfg.mu.Unlock()
	}()

	toggleOverlayVisibility()

	overlayCfg.mu.Lock()
	override := overlayCfg.manualOverride
	overlayCfg.mu.Unlock()
	if override == nil || *override {
		t.Fatal("hiding via hotkey in always policy must record an override, or the next push shows it again")
	}
}

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

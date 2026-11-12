package main

// policyWants reports whether a display policy wants the overlay shown, given
// the room connection and whether GOSpeak is the active app. It is the whole
// automatic decision, kept pure so it is testable without a window.
func policyWants(policy string, connected, appActive bool) bool {
	switch policy {
	case overlayPolicyAuto:
		return connected && !appActive
	case overlayPolicyAlways:
		return connected
	default:
		return false
	}
}

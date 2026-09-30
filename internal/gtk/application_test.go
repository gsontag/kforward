package gtk

import "testing"

// The signals go through the same trampoline as the main loop functions, but
// connected to an instance: creating the application needs neither a display
// nor a session bus, until it is registered.
func TestConnectCallsAtEachEmission(t *testing.T) {
	a := NewApplication("fr.gsontag.kforward.test")
	activations, shutdowns := 0, 0
	a.ConnectActivate(func() { activations++ })
	a.ConnectShutdown(func() { shutdowns++ })

	emit(a.Native(), "activate")
	emit(a.Native(), "activate")

	check(t, "activations", activations, 2)
	// Each function answers its own signal only
	check(t, "shutdowns", shutdowns, 0)
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

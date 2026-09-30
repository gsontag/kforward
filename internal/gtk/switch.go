package gtk

// #include <gtk/gtk.h>
import "C"

import "unsafe"

// Switch is an on/off switch. Its position, Active, and its state, State,
// are distinct: moved by the user, a switch can show that the change is
// still in progress, until State catches up.
type Switch struct {
	Widget
}

// NewSwitch returns a switch, off.
func NewSwitch() *Switch {
	return &Switch{newWidget(C.gtk_switch_new())}
}

func (s *Switch) sw(f func(p *C.GtkSwitch)) {
	s.with(func(p *C.GtkWidget) { f((*C.GtkSwitch)(unsafe.Pointer(p))) })
}

// SetActive sets the position of the switch. It emits state-set, as a
// move by the user does.
func (s *Switch) SetActive(active bool) {
	s.sw(func(p *C.GtkSwitch) { C.gtk_switch_set_active(p, gbool(active)) })
}

// Active returns the position of the switch.
func (s *Switch) Active() bool {
	active := false
	s.sw(func(p *C.GtkSwitch) { active = C.gtk_switch_get_active(p) != C.FALSE })
	return active
}

// SetState sets the state shown under the position.
func (s *Switch) SetState(state bool) {
	s.sw(func(p *C.GtkSwitch) { C.gtk_switch_set_state(p, gbool(state)) })
}

// State returns the state shown under the position.
func (s *Switch) State() bool {
	state := false
	s.sw(func(p *C.GtkSwitch) { state = C.gtk_switch_get_state(p) != C.FALSE })
	return state
}

// ConnectStateSet calls f with the new position, at each move. f returns
// true to set the state itself, later, with SetState; false lets GTK set it
// to the position at once.
func (s *Switch) ConnectStateSet(f func(state bool) bool) {
	s.with(func(p *C.GtkWidget) { connectBool(C.gpointer(p), "state-set", f) })
}

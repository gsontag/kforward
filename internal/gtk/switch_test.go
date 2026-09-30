package gtk

import (
	"fmt"
	"testing"
)

// The window shows the manager's view: the position is what the user asked,
// the state what the forward is. The handler returns true so that GTK does
// not set the state itself.
func TestSwitchDelayedState(t *testing.T) {
	var moves []bool
	var active, stateAfterMove, stateAfterSet bool
	onMain(t, func() {
		s := NewSwitch()
		defer s.obj.release()
		s.ConnectStateSet(func(state bool) bool {
			moves = append(moves, state)
			return true
		})
		s.SetActive(true)
		active, stateAfterMove = s.Active(), s.State()
		s.SetState(true)
		stateAfterSet = s.State()
	})
	check(t, "moves", fmt.Sprint(moves), "[true]")
	check(t, "active", active, true)
	check(t, "state after the move", stateAfterMove, false)
	check(t, "state once set", stateAfterSet, true)
}

func TestSwitchDefaultState(t *testing.T) {
	var state bool
	onMain(t, func() {
		s := NewSwitch()
		defer s.obj.release()
		// false: GTK sets the state to the position
		s.ConnectStateSet(func(bool) bool { return false })
		s.SetActive(true)
		state = s.State()
	})
	check(t, "state", state, true)
}

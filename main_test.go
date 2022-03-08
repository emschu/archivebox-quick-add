package main

import "testing"

func TestDebugModeIsOff(t *testing.T) {
	if isDebug == true {
		t.Error("Debug mode is on!")
	}
}

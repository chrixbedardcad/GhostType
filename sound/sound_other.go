//go:build !windows

package sound

func Init(soundEnabled bool) {}
func PlayStart()             {}
func PlaySuccess()           {}
func PlayError()             {}
func PlayToggle()            {}
func PlayWorking()           {}
func SetEnabled(v bool)      {}

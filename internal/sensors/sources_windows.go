//go:build windows

package sensors

import "golang.org/x/sys/windows"

// New discovers the sources: EMI devices and their channels, then the
// PawnIO driver and module. Discovery never fails; a missing source keeps
// its reason.
func New() *Sources {
	return newSources(readBattery, newEMISensor(nativeEMI{}), newPawnIO())
}

// elevated reports whether this process's token is in the Administrators
// group with the group enabled, which a UAC-filtered token is not.
func elevated() bool {
	admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false
	}
	member, err := windows.Token(0).IsMember(admins)
	return err == nil && member
}

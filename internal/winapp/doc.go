// Package winapp holds the process facts the app starts from: whether the
// token is elevated, the user's SID, the installation this process
// represents (Identity), the one elevation request (TryElevate), and the
// single-instance lock with its restore request. Paths compare the Windows
// way (FullPath, SamePath) on every platform, so the rules built on them are
// tested on Linux too.
//
// The GUI route starts like this; the logon task's launch (--autostart)
// never restores a window, and the elevated copy (--elevation-attempted)
// waits for the lock of the process that started it:
//
//	identity := winapp.Identity()
//	inst, ok, err := winapp.Acquire(identity, route.Handoff)
//	if err != nil || !ok {
//		if err == nil && !route.StartHidden {
//			winapp.RestoreExisting(identity)
//		}
//		return
//	}
//	defer inst.Close()
//	elevation := cli.Decide(route, winapp.IsElevated(), winapp.TryElevate)
//	if elevation.Started {
//		return // the elevated copy takes the lock once this process exits
//	}
//	// ... create the window, then:
//	inst.Listen(func() { /* on the UI thread: show, restore and focus the window */ })
//
// The window's "restart as administrator" button calls TryElevate too and,
// when Started, quits the app so the elevated copy gets the lock.
//
// A protected logon copy (<Program Files>\Power Meter\autostart\<SID>\
// PowerMeter.exe) has the identity of the installation it was copied from,
// so opening that installation by hand restores the running logon
// instance, across integrity levels: the lock and the restore request are
// per-user objects at medium integrity, and the request carries no data.
package winapp

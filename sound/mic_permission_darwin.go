//go:build darwin

package sound

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AVFoundation -framework Foundation

#import <AVFoundation/AVFoundation.h>

// micAuthStatus returns the current microphone authorization status.
//   0 = notDetermined, 1 = restricted, 2 = denied, 3 = authorized
static int micAuthStatus() {
    AVAuthorizationStatus s = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
    return (int)s;
}

// requestMicAccess blocks until the user responds to the permission dialog.
// Returns 1 if granted, 0 if denied.
static int requestMicAccess() {
    __block int granted = 0;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL g) {
        granted = g ? 1 : 0;
        dispatch_semaphore_signal(sem);
    }];
    dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
    return granted;
}
*/
import "C"

import "log/slog"

// CheckMicPermission checks macOS microphone authorization before touching
// CoreAudio. Returns nil if access is granted, an error describing the
// problem otherwise. This prevents SIGABRT under hardened runtime when the
// audio-input entitlement is missing or the user denied permission.
func CheckMicPermission() error {
	status := int(C.micAuthStatus())
	slog.Debug("[mic] macOS authorization status", "status", status)

	switch status {
	case 3: // authorized
		return nil
	case 0: // notDetermined — ask the user
		slog.Info("[mic] Requesting microphone permission...")
		if int(C.requestMicAccess()) == 1 {
			return nil
		}
		return ErrMicPermissionDenied
	case 2: // denied
		return ErrMicPermissionDenied
	case 1: // restricted (parental controls / MDM)
		return ErrMicPermissionDenied
	default:
		return ErrMicPermissionDenied
	}
}

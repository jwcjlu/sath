package errs

import (
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	if !errors.Is(ErrRateLimited, ErrRateLimited) {
		t.Fatalf("ErrRateLimited should be comparable via errors.Is")
	}
	if !errors.Is(ErrContentBlocked, ErrContentBlocked) {
		t.Fatalf("ErrContentBlocked should be comparable via errors.Is")
	}
	if !errors.Is(ErrInternal, ErrInternal) {
		t.Fatalf("ErrInternal should be comparable via errors.Is")
	}
}

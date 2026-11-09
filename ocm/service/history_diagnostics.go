package service

import (
	"context"
	"errors"
	"os"

	"github.com/kimoofey/tui/ocm/history"
	"github.com/kimoofey/tui/ocm/history/diagnostics"
	"github.com/kimoofey/tui/ocm/session"
)

// Classification uses typed error identities only. Raw errors (including native
// stderr, JSON scan values, paths and identifiers) never reach the log encoder.
func diagnosticCode(err error) diagnostics.Code {
	switch {
	case err == nil:
		return diagnostics.OK
	case errors.Is(err, context.Canceled):
		return diagnostics.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return diagnostics.Timeout
	case errors.Is(err, os.ErrPermission):
		return diagnostics.Permission
	case errors.Is(err, os.ErrNotExist):
		return diagnostics.Missing
	case errors.Is(err, history.ErrBinding):
		return diagnostics.BindingMismatch
	case errors.Is(err, history.ErrStale):
		return diagnostics.Stale
	case errors.Is(err, session.ErrHistoryRole):
		return diagnostics.UnknownRole
	case errors.Is(err, session.ErrHistoryEligibility):
		return diagnostics.Eligibility
	case errors.Is(err, session.ErrHistoryDuplicate):
		return diagnostics.Duplicate
	case errors.Is(err, session.ErrUnsupported):
		return diagnostics.Unsupported
	default:
		return diagnostics.Failed
	}
}

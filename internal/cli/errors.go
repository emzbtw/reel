package cli

import (
	"errors"

	"github.com/emzbtw/reel/internal/api"
)

// FormatError turns a config-load or API error into an actionable,
// human-readable message (no raw status codes or JSON bodies).
func FormatError(err error) string {
	switch {
	case errors.Is(err, api.ErrUnauthorized):
		return "Seerr rejected the API key (401 Unauthorized). Check REEL_SEERR_API_KEY or seerr_api_key in your config file."
	case errors.Is(err, api.ErrForbidden):
		return "Seerr denied this action (403 Forbidden). You may only be able to act on your own pending requests unless you have the MANAGE_REQUESTS permission."
	case errors.Is(err, api.ErrNotFound):
		return "Seerr returned 404 Not Found. Check the Seerr URL in your config."
	case errors.Is(err, api.ErrRateLimited):
		return "Seerr rate-limited this request (429 Too Many Requests). Wait a moment and try again."
	default:
		return err.Error()
	}
}

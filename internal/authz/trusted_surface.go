package authz

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

const (
	TrustedSurfaceConfigEnv   = "APOLLO_TRUSTED_SURFACES"
	TrustedSurfaceHeader      = "X-Apollo-Trusted-Surface"
	TrustedSurfaceTokenHeader = "X-Apollo-Trusted-Surface-Token"
)

type TrustedSurface struct {
	Key   string
	Label string
}

type configuredTrustedSurface struct {
	TrustedSurface
	Token string
}

type TrustedSurfaceVerifier struct {
	surfaces map[string]configuredTrustedSurface
}

func NewTrustedSurfaceVerifierFromEnv() *TrustedSurfaceVerifier {
	return NewTrustedSurfaceVerifier(os.Getenv(TrustedSurfaceConfigEnv))
}

func NewTrustedSurfaceVerifier(raw string) *TrustedSurfaceVerifier {
	verifier := &TrustedSurfaceVerifier{
		surfaces: make(map[string]configuredTrustedSurface),
	}

	for _, item := range strings.Split(raw, ",") {
		entry := strings.TrimSpace(item)
		if entry == "" {
			continue
		}

		separator := strings.IndexAny(entry, "=:")
		if separator <= 0 || separator >= len(entry)-1 {
			continue
		}

		key := strings.TrimSpace(entry[:separator])
		token := strings.TrimSpace(entry[separator+1:])
		if key == "" || token == "" {
			continue
		}

		verifier.surfaces[key] = configuredTrustedSurface{
			TrustedSurface: TrustedSurface{
				Key:   key,
				Label: key,
			},
			Token: token,
		}
	}

	return verifier
}

func (v *TrustedSurfaceVerifier) VerifyRequest(r *http.Request) (*TrustedSurface, error) {
	if r == nil {
		return nil, ErrTrustedSurfaceMissing
	}

	token := strings.TrimSpace(r.Header.Get(TrustedSurfaceTokenHeader))
	if token == "" {
		return nil, ErrTrustedSurfaceMissing
	}

	key := strings.TrimSpace(r.Header.Get(TrustedSurfaceHeader))
	if key == "" {
		return nil, ErrTrustedSurfaceKey
	}

	configured, ok := v.surfaces[key]
	if !ok {
		return nil, ErrTrustedSurfaceInvalid
	}
	if subtle.ConstantTimeCompare([]byte(configured.Token), []byte(token)) != 1 {
		return nil, ErrTrustedSurfaceInvalid
	}

	surface := configured.TrustedSurface
	return &surface, nil
}

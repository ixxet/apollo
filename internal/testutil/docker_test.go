package testutil

import (
	"errors"
	"testing"
)

func TestDockerHostResolutionAndOverride(t *testing.T) {
	t.Run("env override wins", func(t *testing.T) {
		t.Setenv("DOCKER_HOST_NAME", "docker.internal")
		originalLookupHost := lookupHost
		lookupHost = func(string) ([]string, error) {
			t.Fatal("lookupHost should not run when DOCKER_HOST_NAME is set")
			return nil, nil
		}
		defer func() {
			lookupHost = originalLookupHost
		}()

		if host := dockerHost(); host != "docker.internal" {
			t.Fatalf("dockerHost() = %q, want docker.internal", host)
		}
	})

	t.Run("host.docker.internal when resolvable", func(t *testing.T) {
		t.Setenv("DOCKER_HOST_NAME", "")
		originalLookupHost := lookupHost
		lookupHost = func(string) ([]string, error) {
			return []string{"192.168.65.2"}, nil
		}
		defer func() {
			lookupHost = originalLookupHost
		}()

		if host := dockerHost(); host != "host.docker.internal" {
			t.Fatalf("dockerHost() = %q, want host.docker.internal", host)
		}
	})

	t.Run("localhost fallback when host.docker.internal does not resolve", func(t *testing.T) {
		t.Setenv("DOCKER_HOST_NAME", "")
		originalLookupHost := lookupHost
		lookupHost = func(string) ([]string, error) {
			return nil, errors.New("lookup failed")
		}
		defer func() {
			lookupHost = originalLookupHost
		}()

		if host := dockerHost(); host != "127.0.0.1" {
			t.Fatalf("dockerHost() = %q, want 127.0.0.1", host)
		}
	})
}

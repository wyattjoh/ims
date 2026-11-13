package providers_test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wyattjoh/ims/internal/image/provider"
	"github.com/wyattjoh/ims/internal/platform/providers"
)

func TestNewConstructionFailures(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(t.TempDir(), "missing-credentials.json"))
	t.Setenv("S3_ENDPOINT", "bad endpoint")
	for _, tt := range []struct {
		name            string
		backends        []string
		secret          string
		signingWithPath bool
		wantError       string
	}{
		{name: "no providers", wantError: "no provider selected"},
		{name: "blank backend", backends: []string{""}, wantError: "cannot parse the backend: cannot be blank"},
		{name: "invalid backend", backends: []string{"a,b,c"}, wantError: "cannot parse the backend: expected form"},
		{name: "duplicate default host", backends: []string{"first-directory", "second-directory"}, wantError: "host default.example already has a provider"},
		{name: "duplicate explicit host", backends: []string{"a.example,first-directory", "a.example,second-directory"}, wantError: "host a.example already has a provider"},
		{name: "duplicate implicit and explicit host", backends: []string{"first-directory", "default.example,second-directory"}, wantError: "host default.example already has a provider"},
		{name: "invalid remote URL", backends: []string{"http://%invalid"}, wantError: "cannot get the origin provider: cannot parse the origin url"},
		{name: "unsupported remote scheme", backends: []string{"ftp://origin.example"}, wantError: "scheme could not be matched"},
		{name: "GCS credentials", backends: []string{"gs://test-bucket"}, wantError: "cannot get the origin provider: cannot get the underlying transport: could not create transport for scheme: cannot create token source"},
		{name: "S3 endpoint", backends: []string{"s3://test-bucket"}, wantError: "cannot get the origin provider: could not create provider for the s3 scheme: cannot create the s3 client"},
		{name: "proxy requires secret", backends: []string{":proxy:"}, signingWithPath: true, wantError: "a signing secret is required"},
		{name: "proxy requires path signing", backends: []string{":proxy:"}, secret: "test-secret", wantError: "a signing with path is required"},
		{name: "partial construction discarded", backends: []string{"valid.example,directory", "invalid.example,ftp://origin.example"}, wantError: "scheme could not be matched"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, err := providers.New(context.Background(), "default.example", tt.backends, "", tt.secret, tt.signingWithPath)
			if p != nil || err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("New() = (%v, %v), want nil providers and error containing %q", p, err, tt.wantError)
			}
		})
	}
}

func TestNewConstructsProviderTypes(t *testing.T) {
	credentials := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(credentials, []byte(`{"type":"authorized_user","client_id":"test-client","client_secret":"test-secret","refresh_token":"test-refresh-token"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credentials)
	t.Setenv("STORAGE_EMULATOR_HOST", "https://storage.example.test")
	t.Setenv("S3_ENDPOINT", "s3.example.test")
	t.Setenv("S3_ACCESS_KEY_ID", "test-key")
	t.Setenv("S3_ACCESS_KEY_SECRET", "test-secret")
	directory := t.TempDir()
	p, err := providers.New(context.Background(), "default.example", []string{
		directory,
		"http.example,http://origin.example.test/images/",
		"https.example,https://origin.example.test/images/",
		"s3.example,s3://test-bucket",
		"gcs.example,gs://test-bucket",
		"proxy.example,:proxy:",
	}, ":memory:", "test-signing-secret", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"default.example", "http.example", "https.example", "s3.example", "gcs.example", "proxy.example"} {
		t.Run(host, func(t *testing.T) {
			got := p.Get(host)
			switch host {
			case "default.example":
				filesystem, ok := got.(*provider.Filesystem)
				if !ok || filesystem.Dir != http.Dir(directory) {
					t.Fatalf("Get(%q) = %#v, want filesystem rooted at %q", host, got, directory)
				}
			case "http.example", "https.example":
				if _, ok := got.(*provider.Origin); !ok {
					t.Fatalf("Get(%q) = %T, want *provider.Origin", host, got)
				}
			case "s3.example":
				if _, ok := got.(*provider.S3); !ok {
					t.Fatalf("Get(%q) = %T, want *provider.S3", host, got)
				}
			case "gcs.example":
				if _, ok := got.(*provider.GCS); !ok {
					t.Fatalf("Get(%q) = %T, want *provider.GCS", host, got)
				}
			case "proxy.example":
				if _, ok := got.(*provider.Proxy); !ok {
					t.Fatalf("Get(%q) = %T, want *provider.Proxy", host, got)
				}
			}
		})
	}
	if p.Get("unknown.example") != nil {
		t.Error("unknown host should not fall back to the default host")
	}
}

func TestRemoteProviderConstructionFailures(t *testing.T) {
	t.Setenv("STORAGE_EMULATOR_HOST", "http://%invalid")
	t.Setenv("S3_ENDPOINT", "bad endpoint")
	for _, tt := range []struct {
		scheme, wantError string
	}{
		{scheme: "gs", wantError: "could not create provider for the gs scheme: cannot create storage client"},
		{scheme: "s3", wantError: "could not create provider for the s3 scheme: cannot create the s3 client"},
		{scheme: "ftp", wantError: "scheme could not be matched"},
		{scheme: "", wantError: "scheme could not be matched"},
	} {
		t.Run(tt.scheme, func(t *testing.T) {
			p, err := providers.GetRemoteProviderClient(context.Background(), &url.URL{Scheme: tt.scheme, Host: "test-bucket"}, http.DefaultTransport)
			if p != nil || err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("GetRemoteProviderClient() = (%v, %v), want error containing %q", p, err, tt.wantError)
			}
		})
	}
}

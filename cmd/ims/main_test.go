package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
	jaeger "github.com/uber/jaeger-client-go"
	"github.com/urfave/cli/v2"
)

func preserveCLIGlobals(t *testing.T) {
	t.Helper()
	tracer := opentracing.GlobalTracer()
	logger := logrus.StandardLogger()
	level, formatter, output := logger.GetLevel(), logger.Formatter, logger.Out
	logger.SetOutput(io.Discard)
	t.Cleanup(func() {
		opentracing.SetGlobalTracer(tracer)
		logger.SetLevel(level)
		logger.SetFormatter(formatter)
		logger.SetOutput(output)
	})
}

func TestSetupTracingWithoutCollector(t *testing.T) {
	preserveCLIGlobals(t)
	previous := opentracing.GlobalTracer()
	tracer, closer := SetupTracing("")
	t.Cleanup(func() {
		if err := closer.Close(); err != nil {
			t.Errorf("close tracer: %v", err)
		}
	})
	if opentracing.GlobalTracer() != previous {
		t.Error("SetupTracing unexpectedly replaced the global tracer")
	}
	span := tracer.StartSpan("not-reported")
	defer span.Finish()
	ctx, ok := span.Context().(jaeger.SpanContext)
	if !ok || !ctx.IsValid() || ctx.IsSampled() {
		t.Errorf("disabled reporter context = %v, want a valid unsampled Jaeger context", span.Context())
	}
	carrier := opentracing.TextMapCarrier{}
	if err := tracer.Inject(span.Context(), opentracing.TextMap, carrier); err != nil {
		t.Fatal(err)
	}
	if _, err := tracer.Extract(opentracing.TextMap, carrier); err != nil {
		t.Errorf("extract injected context: %v", err)
	}
}

func TestSetupTracingWithCollector(t *testing.T) {
	preserveCLIGlobals(t)
	type report struct {
		method, path, contentType string
		size                      int
	}
	reports := make(chan report, 1)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read tracing report: %v", err)
		}
		select {
		case reports <- report{r.Method, r.URL.Path, r.Header.Get("Content-Type"), len(body)}:
		default:
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(collector.Close)
	tracer, closer := SetupTracing(collector.URL + "/api/traces")
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = closer.Close()
		}
	})
	span := tracer.StartSpan("reported")
	ctx, ok := span.Context().(jaeger.SpanContext)
	if !ok || !ctx.IsSampled() {
		t.Errorf("collector context = %v, want sampled Jaeger context", span.Context())
	}
	span.Finish()
	if err := closer.Close(); err != nil {
		t.Fatalf("close tracer: %v", err)
	}
	closed = true
	select {
	case got := <-reports:
		if got.method != http.MethodPost || got.path != "/api/traces" || got.contentType != "application/x-thrift" || got.size == 0 {
			t.Errorf("unexpected tracing report: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no trace report received after closing reporter")
	}
}

func serveActionContext(t *testing.T, args []string) *cli.Context {
	t.Helper()
	application := newCLIApp()
	application.Writer, application.ErrWriter = io.Discard, io.Discard
	var parsed *cli.Context
	application.Action = func(c *cli.Context) error {
		parsed = c
		return nil
	}
	arguments := append([]string{"ims", "--listen-addr", "127.0.0.1:65536", "--disable-metrics"}, args...)
	if err := application.Run(arguments); err != nil {
		t.Fatal(err)
	}
	if parsed == nil {
		t.Fatal("CLI action was not called")
	}
	return parsed
}

func TestServeActionErrors(t *testing.T) {
	for _, tt := range []struct {
		name      string
		args      []string
		wantError string
		debugJSON bool
	}{
		{name: "working directory default", wantError: "could not listen on the address for http traffic"},
		{name: "invalid backend", args: []string{"--backend", ",bad"}, wantError: "cannot create providers: cannot parse the backend"},
		{name: "proxy requires signing", args: []string{"--backend", ":proxy:"}, wantError: "a signing secret is required"},
		{name: "proxy requires path signing", args: []string{"--backend", ":proxy:", "--signing-secret", "test-secret"}, wantError: "a signing with path is required"},
		{name: "configured signed proxy", args: []string{
			"--backend", ":proxy:", "--signing-secret", "test-secret", "--signing-with-path",
			"--debug", "--json", "--origin-cache", ":memory:", "--timeout", "1m", "--cors-domain", "https://images.test",
		}, wantError: "could not listen on the address for http traffic", debugJSON: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			preserveCLIGlobals(t)
			logrus.SetLevel(logrus.InfoLevel)
			logrus.SetFormatter(&logrus.TextFormatter{})
			err := ServeAction(serveActionContext(t, tt.args))
			exitError, ok := err.(cli.ExitCoder)
			if !ok || exitError.ExitCode() != 1 || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("ServeAction() error = %v, want CLI exit 1 containing %q", err, tt.wantError)
			}
			if tt.debugJSON {
				if logrus.GetLevel() != logrus.DebugLevel {
					t.Error("debug flag did not enable debug logging")
				}
				if _, ok := logrus.StandardLogger().Formatter.(*logrus.JSONFormatter); !ok {
					t.Error("json flag did not enable JSON logging")
				}
			}
		})
	}
}

func TestCLIBackendParsing(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for _, tt := range []struct {
		name      string
		backends  []string
		wantError string
	}{
		{name: "named backend", backends: []string{"images.test," + first}, wantError: "could not listen on the address for http traffic"},
		{name: "repeated distinct named backends", backends: []string{"first.test," + first, "second.test," + second}, wantError: "could not listen on the address for http traffic"},
		{name: "repeated named host retains both values", backends: []string{"images.test," + first, "images.test," + second}, wantError: "host images.test already has a provider attached to it"},
		{name: "default and named backend", backends: []string{first, "images.test," + second}, wantError: "could not listen on the address for http traffic"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			preserveCLIGlobals(t)
			application := newCLIApp()
			// Let App.Run return the action's CLI exit error, not exit the test process.
			application.ExitErrHandler = func(*cli.Context, error) {}
			application.Action = func(c *cli.Context) error {
				if got := c.StringSlice(flagBackend); !slices.Equal(got, tt.backends) {
					t.Errorf("parsed backends = %q, want %q", got, tt.backends)
				}
				// Match ServeAction's normalization: CORS must still support commas,
				// whitespace trimming, and accumulation across repeated flags.
				wantCORS := []string{"https://one.test", "https://two.test", "https://three.test"}
				if got := splitCORSDomains(c.StringSlice(flagCORSDomain)); !slices.Equal(got, wantCORS) {
					t.Errorf("CORS domains = %q, want %q", got, wantCORS)
				}
				return ServeAction(c)
			}
			args := []string{
				"ims", "--listen-addr", "127.0.0.1:65536", "--disable-metrics",
				"--cors-domain", " https://one.test , https://two.test ", "--cors-domain", "https://three.test",
			}
			for _, backend := range tt.backends {
				args = append(args, "--backend", backend)
			}
			err := application.Run(args)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("real CLI error = %v, want %q", err, tt.wantError)
			}
		})
	}
}

func TestCLICORSParsing(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want []string
	}{
		{name: "absent"},
		{name: "comma separated", args: []string{"--cors-domain", "https://one.test,https://two.test"}, want: []string{"https://one.test", "https://two.test"}},
		{name: "repeated and trimmed", args: []string{"--cors-domain", " https://one.test , https://two.test ", "--cors-domain", " https://three.test "}, want: []string{"https://one.test", "https://two.test", "https://three.test"}},
		{name: "blank entries retained", args: []string{"--cors-domain", ", https://one.test,"}, want: []string{"", "https://one.test", ""}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			application := newCLIApp()
			application.Action = func(c *cli.Context) error {
				if got := splitCORSDomains(c.StringSlice(flagCORSDomain)); !slices.Equal(got, tt.want) {
					t.Errorf("normalized CORS domains = %q, want %q", got, tt.want)
				}
				return nil
			}
			if err := application.Run(append([]string{"ims"}, tt.args...)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMainInformationalFlags(t *testing.T) {
	for _, tt := range []struct {
		name string
		arg  string
		want []string
	}{
		{name: "help", arg: "--help", want: []string{"Image Manipulation Server", "--backend", "--signing-with-path", "--tracing-uri", defaultListenAddr}},
		{name: "version", arg: "--version", want: []string{"ims version " + version + ", commit " + commit + ", built at " + date}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// A file avoids pipe-buffer limits when capturing the real main's output.
			output, err := os.CreateTemp(t.TempDir(), "cli-output-*")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = output.Close() })
			previousArgs, previousStdout := os.Args, os.Stdout
			os.Args, os.Stdout = []string{"ims", tt.arg}, output
			t.Cleanup(func() { os.Args, os.Stdout = previousArgs, previousStdout })
			main()
			if _, err := output.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(output)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tt.want {
				if !strings.Contains(string(body), want) {
					t.Errorf("output = %q, want substring %q", body, want)
				}
			}
		})
	}
}

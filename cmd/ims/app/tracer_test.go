package app_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/wyattjoh/ims/cmd/ims/app"
)

func useMockTracer(t *testing.T) *mocktracer.MockTracer {
	t.Helper()
	previous := opentracing.GlobalTracer()
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)
	t.Cleanup(func() { opentracing.SetGlobalTracer(previous) })
	return tracer
}

func TestTracer(t *testing.T) {
	for _, tt := range []struct {
		name      string
		method    string
		parent    bool
		malformed bool
	}{
		{name: "root span", method: http.MethodGet},
		{name: "propagated parent", method: http.MethodPost, parent: true},
		{name: "invalid carrier ignored", method: http.MethodHead, malformed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tracer := useMockTracer(t)
			request := httptest.NewRequest(tt.method, "http://images.test/photo.png?width=20", nil)
			type contextKey struct{}
			request = request.WithContext(context.WithValue(request.Context(), contextKey{}, "preserved"))
			var parent *mocktracer.MockSpan
			if tt.parent {
				parent = tracer.StartSpan("upstream").(*mocktracer.MockSpan)
				parent.SetBaggageItem("tenant", "example")
				t.Cleanup(parent.Finish)
				if err := tracer.Inject(parent.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(request.Header)); err != nil {
					t.Fatal(err)
				}
			}
			if tt.malformed {
				request.Header.Set("Mockpfx-Ids-Traceid", "not-a-number")
				request.Header.Set("Mockpfx-Ids-Spanid", "123")
			}

			response := httptest.NewRecorder()
			var downstreamSpan opentracing.Span
			calls := 0
			app.Tracer(response, request, func(w http.ResponseWriter, r *http.Request) {
				calls++
				downstreamSpan = opentracing.SpanFromContext(r.Context())
				if downstreamSpan == nil {
					t.Fatal("downstream request has no tracing span")
				}
				if len(tracer.FinishedSpans()) != 0 {
					t.Error("span finished before downstream handler returned")
				}
				if r.Context().Value(contextKey{}) != "preserved" || r.URL != request.URL || r.Method != request.Method {
					t.Error("middleware did not preserve the request and context values")
				}
				w.Header().Set("X-Downstream", "yes")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("image"))
			})
			if calls != 1 || response.Code != http.StatusCreated || response.Body.String() != "image" || response.Header().Get("X-Downstream") != "yes" {
				t.Errorf("downstream response not preserved: calls=%d status=%d body=%q", calls, response.Code, response.Body.String())
			}
			if opentracing.SpanFromContext(request.Context()) != nil {
				t.Error("middleware modified the original request context")
			}
			finished := tracer.FinishedSpans()
			if len(finished) != 1 {
				t.Fatalf("finished spans = %d, want 1", len(finished))
			}
			span := finished[0]
			if span != downstreamSpan || span.OperationName != "HTTP "+tt.method || span.FinishTime.IsZero() {
				t.Errorf("unexpected finished span: %v", span)
			}
			for key, want := range map[string]interface{}{
				string(ext.HTTPMethod): tt.method,
				string(ext.HTTPUrl):    request.URL.String(),
				string(ext.SpanKind):   ext.SpanKindRPCServerEnum,
			} {
				if got := span.Tag(key); got != want {
					t.Errorf("span tag %q = %v, want %v", key, got, want)
				}
			}
			if parent == nil {
				if span.ParentID != 0 {
					t.Errorf("root span has parent ID %d", span.ParentID)
				}
				return
			}
			if span.ParentID != parent.SpanContext.SpanID || span.SpanContext.TraceID != parent.SpanContext.TraceID || span.BaggageItem("tenant") != "example" {
				t.Errorf("parent context not propagated: parent=%v child=%v", parent, span)
			}
		})
	}
}

func TestTracerFinishesSpanOnPanic(t *testing.T) {
	tracer := useMockTracer(t)
	panicValue := &struct{}{}
	func() {
		defer func() {
			if got := recover(); got != panicValue {
				t.Errorf("panic = %v, want downstream panic %v", got, panicValue)
			}
		}()
		app.Tracer(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/image", nil), func(http.ResponseWriter, *http.Request) {
			panic(panicValue)
		})
	}()
	if finished := tracer.FinishedSpans(); len(finished) != 1 || finished[0].FinishTime.IsZero() {
		t.Errorf("expected one finished span after panic, got %v", finished)
	}
}

package privatecaptcha

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

const (
	solutionsCount    = 16
	solutionLength    = 8
	traceIDContextKey = "tid"
)

type contextHandler struct {
	slog.Handler
}

func traceIDAttr(tid string) slog.Attr {
	return slog.Attr{
		Key:   "traceID",
		Value: slog.StringValue(tid),
	}
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx != nil {
		if tid, ok := ctx.Value(traceIDContextKey).(string); ok && (len(tid) > 0) {
			r.AddAttrs(traceIDAttr(tid))
		}
	}

	return h.Handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{h.Handler.WithGroup(name)}
}

func setupTraceLogs() {
	opts := &slog.HandlerOptions{
		Level: levelTrace,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	ctxHandler := &contextHandler{handler}
	logger := slog.New(ctxHandler)
	slog.SetDefault(logger)
}

func init() {
	setupTraceLogs()
}

func fetchTestPuzzle(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.privatecaptcha.com/puzzle?sitekey=aaaaaaaabbbbccccddddeeeeeeeeeeee", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Origin", "not.empty")

	slog.Log(ctx, levelTrace, "About to send puzzle request")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Log(ctx, levelTrace, "Failed to send HTTP request", "path", req.URL.Path, "method", req.Method, errAttr(err))
		return nil, err
	}

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Log(ctx, levelTrace, "Failed to read puzzle response", errAttr(err))
		return nil, err
	}

	slog.Log(ctx, levelTrace, "Received puzzle", "puzzle", len(data))

	return data, nil
}

func TestStubPuzzle(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.TODO(), traceIDContextKey, t.Name())

	puzzle, err := fetchTestPuzzle(ctx)
	if err != nil {
		t.Fatal(err)
	}

	client, err := NewClient(Configuration{
		APIKey: os.Getenv("PC_API_KEY"),
	})
	if err != nil {
		t.Fatal(err)
	}

	emptySolutionsBytes := make([]byte, solutionsCount*solutionLength)
	solutionsStr := base64.StdEncoding.EncodeToString(emptySolutionsBytes)
	payload := fmt.Sprintf("%s.%s", solutionsStr, string(puzzle))

	output, err := client.Verify(ctx, VerifyInput{Solution: payload})
	if err != nil {
		t.Fatal(err)
	}

	if !output.Success || (output.Code != TestPropertyError) {
		t.Errorf("Unexpected result (%v) or error (%v)", output.Success, output.Code)
	}
}

func TestVerifyError(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.TODO(), traceIDContextKey, t.Name())

	puzzle, err := fetchTestPuzzle(ctx)
	if err != nil {
		t.Fatal(err)
	}

	client, err := NewClient(Configuration{
		APIKey: os.Getenv("PC_API_KEY"),
	})
	if err != nil {
		t.Fatal(err)
	}

	emptySolutionsBytes := make([]byte, solutionsCount*solutionLength/2)
	solutionsStr := base64.StdEncoding.EncodeToString(emptySolutionsBytes)
	payload := fmt.Sprintf("%s.%s", solutionsStr, string(puzzle))

	output, err := client.Verify(ctx, VerifyInput{Solution: payload})
	if err != nil {
		t.Fatal(err)
	}

	if output.Success || (output.Code != ParseResponseError) {
		t.Errorf("Unexpected result (%v) or error (%v)", output.Success, output.Code)
	}
}

func TestVerifyEmptySolution(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.TODO(), traceIDContextKey, t.Name())

	client, err := NewClient(Configuration{
		APIKey: os.Getenv("PC_API_KEY"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.Verify(ctx, VerifyInput{}); err != errEmtpySolution {
		t.Fatal("Should not proceed on empty solution")
	}
}

func TestRetryBackoff(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.TODO(), traceIDContextKey, t.Name())

	client, err := NewClient(Configuration{
		APIKey: os.Getenv("PC_API_KEY"),
		Domain: "does-not-exist.qwerty12345-asdfjkl.net",
		Client: &http.Client{Timeout: 1 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}

	input := VerifyInput{
		Solution:          "asdf",
		MaxBackoffSeconds: 1,
		Attempts:          4,
	}

	response, err := client.Verify(ctx, input)

	if err == nil {
		t.Fatal("Managed to verify invalid domain")
	}

	if response.attempt != input.Attempts {
		t.Fatal("Didn't go through all attempts")
	}
}

func TestCustomFormField(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.TODO(), traceIDContextKey, t.Name())

	puzzle, err := fetchTestPuzzle(ctx)
	if err != nil {
		t.Fatal(err)
	}

	customFieldName := "my-custom-captcha-field"
	client, err := NewClient(Configuration{
		APIKey:    os.Getenv("PC_API_KEY"),
		FormField: customFieldName,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create a valid test payload (using empty solutions for test property)
	emptySolutionsBytes := make([]byte, solutionsCount*solutionLength)
	solutionsStr := base64.StdEncoding.EncodeToString(emptySolutionsBytes)
	payload := fmt.Sprintf("%s.%s", solutionsStr, string(puzzle))

	// Create form data with our custom field name
	formData := url.Values{}
	formData.Set(customFieldName, payload)

	// Create HTTP request with form data
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.PostForm = formData

	// Verify that VerifyRequest reads from the custom form field
	if err := client.VerifyRequest(ctx, req); err != nil {
		t.Fatal(err)
	}

	// Also test that it doesn't work with the default field name
	defaultFormData := url.Values{}
	defaultFormData.Set(DefaultFormField, payload)

	defaultReq := httptest.NewRequest(http.MethodPost, "/test", nil)
	defaultReq.PostForm = defaultFormData

	// This should fail because the client is configured to use the custom field
	if err := client.VerifyRequest(ctx, defaultReq); err != errEmtpySolution {
		t.Errorf("Unexpected error: %v", err)
	}
}

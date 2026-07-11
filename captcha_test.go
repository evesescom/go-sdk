package eveses

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCaptchaSolve_PollsUntilReady(t *testing.T) {
	var gotSolvePath, gotResultPath, gotMethod string
	var gotBody map[string]any
	call := 0

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/captcha/solve"):
			gotSolvePath = r.URL.Path
			gotMethod = r.Method
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"task_id":7,"status":"queued","price_micro_usd":3392,"retry_after":0}`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/captcha/result/"):
			gotResultPath = r.URL.Path
			call++
			if call == 1 {
				_, _ = w.Write([]byte(`{"status":"processing","retry_after":0}`))
			} else {
				_, _ = w.Write([]byte(`{"status":"ready","solution":"TOK","retry_after":0}`))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	res, err := client.Captcha.Solve(context.Background(), "RecaptchaV2TaskProxyless", map[string]any{
		"websiteURL": "x", "websiteKey": "k",
	}, nil)
	if err != nil {
		t.Fatalf("Solve error: %v", err)
	}
	if res.TaskID != 7 || res.Status != "ready" || res.Solution != "TOK" || res.PriceMicroUSD != 3392 {
		t.Errorf("unexpected result: %#v", res)
	}
	if gotMethod != http.MethodPost || gotSolvePath != "/api/account/captcha/solve" {
		t.Errorf("solve request = %s %s", gotMethod, gotSolvePath)
	}
	if gotResultPath != "/api/account/captcha/result/7" {
		t.Errorf("result path = %s", gotResultPath)
	}
	if gotBody["type"] != "RecaptchaV2TaskProxyless" {
		t.Errorf("body type = %v", gotBody["type"])
	}
}

func TestCaptchaSolve_FailedTask(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/captcha/solve") {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"task_id":9,"status":"queued","retry_after":0}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"failed","error":"ERROR_CAPTCHA_UNSOLVABLE","retry_after":0}`))
	})

	_, err := client.Captcha.Solve(context.Background(), "ImageToTextTask", nil, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ERROR_CAPTCHA_UNSOLVABLE") {
		t.Errorf("error = %v, want ERROR_CAPTCHA_UNSOLVABLE", err)
	}
}

func TestCaptchaSolve_SendsIdempotencyKey(t *testing.T) {
	var gotIdempotency string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotIdempotency = r.Header.Get("Idempotency-Key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"task_id":3,"status":"ready","solution":"A","retry_after":0}`))
	})

	res, err := client.Captcha.Solve(context.Background(), "ImageToTextTask", nil, &CaptchaSolveParams{
		IdempotencyKey: "idem-c",
	})
	if err != nil {
		t.Fatalf("Solve error: %v", err)
	}
	if res.Solution != "A" {
		t.Errorf("solution = %q, want A", res.Solution)
	}
	if gotIdempotency != "idem-c" {
		t.Errorf("Idempotency-Key = %q, want idem-c", gotIdempotency)
	}
}

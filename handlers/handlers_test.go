package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
)

func waitForServer(t *testing.T, url string) {
	t.Helper()

	client := &http.Client{
		Timeout: 200 * time.Millisecond,
	}

	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("server did not become ready: %s", url)
}

func newClient(t *testing.T) *http.Client {
	t.Helper()

	if err := godotenv.Load("../.env"); err != nil {
		t.Fatal(err)
	}

	client := &http.Client{}
	client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		req.Header.Set("X-API-Key", os.Getenv("API_KEY"))
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:89.0) Gecko/20100101 Firefox/89.0")
		return http.DefaultTransport.RoundTrip(req)
	})

	return client
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func getResp(t *testing.T, client *http.Client, addr string) (int, map[string]any) {
	resp, err := client.Get(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var js map[string]any
	_ = json.Unmarshal(body, &js)
	return resp.StatusCode, js
}

func TestServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "make", "run", "--directory", "..")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		cancel()
		_ = cmd.Wait()
	})

	waitForServer(t, "http://127.0.0.1:1729/healthz")
	client := newClient(t)

	status, resp := getResp(t, client, "http://localhost:3213?ip=85.214.143.21&mode=short")
	assert.Equal(t, status, 200)
	assert.Equal(t, resp["ip"], "85.214.143.21")
	assert.Equal(t, resp["country"], "Germany")

	status, resp = getResp(t, client, "http://localhost:3213?ip=85.214.143.21&mode=echoip")
	assert.Equal(t, resp["asn"], "A6724")
	assert.Equal(t, resp["user_agent"].(map[string]interface{})["product"], "Mozilla")

	status, resp = getResp(t, client, "http://localhost:3213?ip=1.1.1.1&mode=full")
	assert.Equal(t, resp["registered_country"].(map[string]interface{})["iso_code"], "AU")
	assert.Equal(t, resp["hostname"], "one.one.one.one")

	status, resp = getResp(t, client, "http://localhost:3213/portcheck?ip=8.8.8.8&port=53")
	assert.Equal(t, resp["reachable"], true)
	assert.Equal(t, resp["port"], 53.0)
}

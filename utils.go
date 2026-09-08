package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/matthewhartstonge/argon2"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func downloadFile(ctx context.Context, url, filename string) error {
	client := &http.Client{
		Timeout: 0, // no overall timeout; context controls cancellation
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

func checkAuth(key string, provided string) bool {
	ok, err := argon2.VerifyEncoded([]byte(provided), []byte(key))
	if err != nil {
		return false
	}
	return ok
}

func (writer logWriter) Write(bytes []byte) (int, error) {
	return fmt.Print(Grey + "[" + time.Now().Local().Format("2006-01-02T15:04:05.000Z") + "]" + Reset + " " + string(bytes))
}

func logText(clientIP string, mode Mode, queryIP string, attemptType uint) string {
	var prefix string
	var color string
	var modeText string
	var queryText string

	switch attemptType {
	case GoodAttempt:
		prefix = "Accessed"
		color = Green
	case Unauthorized:
		prefix = "Unauthorized attempt"
		color = Red
	case BadAttempt:
		prefix = "Bad request"
		color = Red
	}

	switch mode {
	case Full:
		modeText = ", mode: Full"
	case IPOnly:
		modeText = ", mode: IPOnly"
	}

	if queryIP != clientIP {
		queryText = ", query: " + queryIP
	}

	return fmt.Sprintf("%s%s from %s%s%s%s", color, prefix, clientIP, modeText, queryText, Reset)
}

package utils

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestValidateRemoteFileURLRejectsUnsafeHosts(t *testing.T) {
	for _, rawURL := range []string{
		"ftp://example.com/file.txt",
		"http:///file.txt",
		"http://localhost/file.txt",
		"http://127.0.0.1/file.txt",
		"http://10.0.0.1/file.txt",
		"http://169.254.1.1/file.txt",
	} {
		t.Run(rawURL, func(t *testing.T) {
			if err := ValidateRemoteFileURL(rawURL); err == nil {
				t.Fatal("ValidateRemoteFileURL() expected an error")
			}
		})
	}
}

func TestDownloadRemoteFile(t *testing.T) {
	oldClient := RemoteFileHTTPClient
	t.Cleanup(func() { RemoteFileHTTPClient = oldClient })
	RemoteFileHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Body:          io.NopCloser(strings.NewReader("hello")),
			ContentLength: 5,
			Header:        make(http.Header),
			Request:       req,
		}, nil
	})}

	destPath := filepath.Join(t.TempDir(), "file.txt")
	if err := DownloadRemoteFile("http://93.184.216.34/file.txt", destPath, 10); err != nil {
		t.Fatalf("DownloadRemoteFile() error = %v", err)
	}
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("content = %q, want hello", content)
	}
}

func TestDownloadRemoteFileRejectsStatusAndSize(t *testing.T) {
	oldClient := RemoteFileHTTPClient
	t.Cleanup(func() { RemoteFileHTTPClient = oldClient })

	RemoteFileHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusNotFound,
			Status:        "404 Not Found",
			Body:          io.NopCloser(strings.NewReader("missing")),
			ContentLength: 7,
			Header:        make(http.Header),
			Request:       req,
		}, nil
	})}
	if err := DownloadRemoteFile("http://93.184.216.34/missing.txt", filepath.Join(t.TempDir(), "missing.txt"), 10); err == nil {
		t.Fatal("DownloadRemoteFile(status) expected an error")
	}

	RemoteFileHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Body:          io.NopCloser(strings.NewReader("too large")),
			ContentLength: 9,
			Header:        make(http.Header),
			Request:       req,
		}, nil
	})}
	if err := DownloadRemoteFile("http://93.184.216.34/large.txt", filepath.Join(t.TempDir(), "large.txt"), 3); err == nil {
		t.Fatal("DownloadRemoteFile(size) expected an error")
	}

	RemoteFileHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Body:          io.NopCloser(strings.NewReader("stream too large")),
			ContentLength: -1,
			Header:        make(http.Header),
			Request:       req,
		}, nil
	})}
	destPath := filepath.Join(t.TempDir(), "stream.txt")
	if err := DownloadRemoteFile("http://93.184.216.34/stream.txt", destPath, 3); err == nil {
		t.Fatal("DownloadRemoteFile(stream size) expected an error")
	}
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Fatalf("oversized stream should remove destination, stat err = %v", err)
	}
}

func TestDownloadRemoteFileRejectsUnsafeRedirect(t *testing.T) {
	oldClient := RemoteFileHTTPClient
	t.Cleanup(func() { RemoteFileHTTPClient = oldClient })
	calls := 0
	RemoteFileHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls > 1 {
			t.Fatal("unsafe redirect should be blocked before the second request")
		}
		return &http.Response{
			StatusCode: http.StatusFound,
			Status:     "302 Found",
			Body:       io.NopCloser(strings.NewReader("redirect")),
			Header: http.Header{
				"Location": []string{"http://127.0.0.1/file.txt"},
			},
			Request: req,
		}, nil
	})}

	if err := DownloadRemoteFile("http://93.184.216.34/redirect.txt", filepath.Join(t.TempDir(), "redirect.txt"), 10); err == nil {
		t.Fatal("DownloadRemoteFile(unsafe redirect) expected an error")
	}
}

func TestRemoteFileClientAndDialerGuards(t *testing.T) {
	oldClient := RemoteFileHTTPClient
	t.Cleanup(func() { RemoteFileHTTPClient = oldClient })
	RemoteFileHTTPClient = nil
	client := remoteFileHTTPClient()
	if client.Timeout == 0 || client.Transport == nil {
		t.Fatalf("remoteFileHTTPClient() = %+v, want default timeout and transport", client)
	}
	if _, err := remoteFileDialContext(context.Background(), "tcp", "bad-address"); err == nil {
		t.Fatal("remoteFileDialContext(bad address) expected error")
	}
	if _, err := remoteFileDialContext(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("remoteFileDialContext(loopback) expected error")
	}
}

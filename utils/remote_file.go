package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const DefaultMaxRemoteFileBytes int64 = 50 << 20

var RemoteFileHTTPClient = newRemoteFileHTTPClient()

func DownloadRemoteFile(rawURL, destPath string, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxRemoteFileBytes
	}
	if err := ValidateRemoteFileURL(rawURL); err != nil {
		return err
	}

	client := remoteFileHTTPClient()
	resp, err := client.Get(rawURL)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("remote file returned status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return fmt.Errorf("remote file exceeds %d bytes", maxBytes)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	n, err := io.Copy(out, io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		_ = os.Remove(destPath)
		return err
	}
	if n > maxBytes {
		_ = os.Remove(destPath)
		return fmt.Errorf("remote file exceeds %d bytes", maxBytes)
	}
	return nil
}

func newRemoteFileHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext:           remoteFileDialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

func remoteFileHTTPClient() http.Client {
	if RemoteFileHTTPClient == nil {
		return *newRemoteFileHTTPClient()
	}
	client := *RemoteFileHTTPClient
	prevCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := ValidateRemoteFileURL(req.URL.String()); err != nil {
			return err
		}
		if prevCheckRedirect != nil {
			return prevCheckRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return client
}

func remoteFileDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var dialIP net.IP
	for _, addr := range ips {
		if isBlockedRemoteFileIP(addr.IP) {
			return nil, fmt.Errorf("remote file URL host is not allowed")
		}
		if dialIP == nil {
			dialIP = addr.IP
		}
	}
	if dialIP == nil {
		return nil, fmt.Errorf("remote file URL host could not be resolved")
	}
	dialer := &net.Dialer{}
	return dialer.DialContext(ctx, network, net.JoinHostPort(dialIP.String(), port))
}

func ValidateRemoteFileURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("remote file URL must use http or https")
	}
	host := strings.TrimSuffix(strings.ToLower(parsedURL.Hostname()), ".")
	if host == "" {
		return fmt.Errorf("remote file URL host is required")
	}
	if host == "localhost" {
		return fmt.Errorf("remote file URL host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedRemoteFileIP(ip) {
			return fmt.Errorf("remote file URL host is not allowed")
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if isBlockedRemoteFileIP(ip) {
			return fmt.Errorf("remote file URL host is not allowed")
		}
	}
	return nil
}

func isBlockedRemoteFileIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

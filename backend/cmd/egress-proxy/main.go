// Command egress-proxy is a minimal HTTP/HTTPS forward proxy that only
// allows CONNECT/requests to a fixed set of whitelisted domains. It is the
// enforcement point for FEAT-006's agent sandbox egress whitelist: sandbox
// containers sit on an internal (no direct internet route) Docker network
// and are pointed at this proxy via HTTP_PROXY/HTTPS_PROXY, so the only way
// out is through here.
//
// The whitelist is passed in once via the ALLOWED_DOMAINS env var at
// container creation - see AgentService.ensureEgressProxy, which recreates
// this container whenever the whitelist changes.
package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	port := getEnv("PORT", "8888")
	allowed := parseDomains(getEnv("ALLOWED_DOMAINS", ""))

	srv := &http.Server{
		Addr:        ":" + port,
		Handler:     &proxyHandler{allowed: allowed},
		ReadTimeout: 30 * time.Second,
		// No WriteTimeout: CONNECT tunnels are long-lived streams.
	}

	log.Printf("egress-proxy listening on :%s, %d domain(s) allowed", port, len(allowed))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("egress-proxy: %v", err)
	}
}

type proxyHandler struct {
	allowed map[string]bool
}

func (p *proxyHandler) isAllowed(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return p.allowed[host]
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	p.handleForward(w, r)
}

// handleConnect services HTTPS tunnels: "CONNECT host:port HTTP/1.1". This
// is the path virtually every AI provider API client uses.
func (p *proxyHandler) handleConnect(w http.ResponseWriter, r *http.Request) {
	if !p.isAllowed(r.Host) {
		log.Printf("egress-proxy: DENY CONNECT %s", r.Host)
		http.Error(w, "domain not allowed", http.StatusForbidden)
		return
	}

	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		destConn.Close()
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		destConn.Close()
		return
	}

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		clientConn.Close()
		destConn.Close()
		return
	}
	log.Printf("egress-proxy: ALLOW CONNECT %s", r.Host)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(destConn, clientConn)
		destConn.Close()
	}()
	go func() {
		defer wg.Done()
		io.Copy(clientConn, destConn)
		clientConn.Close()
	}()
	wg.Wait()
}

// hopByHopHeaders must not be forwarded verbatim, per RFC 7230 6.1.
var hopByHopHeaders = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

// handleForward services plain absolute-URI HTTP requests (rare in
// practice for these agent CLIs, which talk HTTPS, but handled for
// completeness/robustness).
func (p *proxyHandler) handleForward(w http.ResponseWriter, r *http.Request) {
	if r.URL.Host == "" {
		http.Error(w, "this is a forward proxy, not a regular server", http.StatusBadRequest)
		return
	}
	if !p.isAllowed(r.URL.Host) {
		log.Printf("egress-proxy: DENY %s %s", r.Method, r.URL.Host)
		http.Error(w, "domain not allowed", http.StatusForbidden)
		return
	}

	outReq := r.Clone(context.Background())
	outReq.RequestURI = ""
	for _, h := range hopByHopHeaders {
		outReq.Header.Del(h)
	}

	resp, err := http.DefaultTransport.RoundTrip(outReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	log.Printf("egress-proxy: ALLOW %s %s", r.Method, r.URL.Host)

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func parseDomains(csv string) map[string]bool {
	out := make(map[string]bool)
	for _, d := range strings.Split(csv, ",") {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		out[d] = true
	}
	return out
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

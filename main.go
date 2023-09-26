// Simple HTTP proxy server that connects to a SOCKS proxy server and passes
// traffic through it.
//
// Inspired by an article on Eli's Bendersky blog
// [https://eli.thegreenplace.net/2022/go-and-proxy-servers-part-1-http-proxies/]
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
// Note: this may be out of date, see RFC 7230 Section 6.1
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",      // canonical version of "TE"
	"Trailer", // spelling per https://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func removeHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

// removeConnectionHeaders removes hop-by-hop headers listed in the "Connection"
// header of h. See RFC 7230, section 6.1
func removeConnectionHeaders(h http.Header) {
	for _, f := range h["Connection"] {
		for _, sf := range strings.Split(f, ",") {
			if sf = strings.TrimSpace(sf); sf != "" {
				h.Del(sf)
			}
		}
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

type forwardProxy struct {
	SocksServer   string
	SocksUser     string
	SocksPassword string
}

func (p *forwardProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// The "Host:" header is promoted to Request.Host and is removed from
	// request.Header by net/http, so we print it out explicitly.
	log.Printf("%s\t%s\t%s\tHost: %s\n", req.RemoteAddr, req.Method, req.URL, req.Host)
	log.Println("\t", req.Header)

	if req.URL.Scheme == "" {
		if req.URL.Port() == "443" {
			req.URL.Scheme = "https"
		} else {
			req.URL.Scheme = "http"
		}
	}

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		msg := "unsupported protocol scheme " + req.URL.Scheme
		http.Error(w, msg, http.StatusBadRequest)
		log.Println(msg)
		return
	}

	if req.Method == http.MethodConnect {
		p.proxyConnect(w, req)
		return
	}

	client, clientErr := p.getHTTPClient()
	if clientErr != nil {
		msg := fmt.Sprintf("failed create http client: %v", clientErr)
		http.Error(w, msg, http.StatusInternalServerError)
		log.Println(msg)
		return
	}

	// When a http.Request is sent through a http.Client, RequestURI should not
	// be set (see documentation of this field).
	req.RequestURI = ""

	removeHopHeaders(req.Header)
	removeConnectionHeaders(req.Header)

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		appendHostToXForwardHeader(req.Header, clientIP)
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Server Error", http.StatusInternalServerError)
		log.Printf("ServeHTTP request error: %+v", err)
	}
	defer func() {
		if resp == nil || resp.Body == nil {
			return
		}
		closeErr := resp.Body.Close()
		if closeErr != nil {
			log.Printf("ServeHTTP close body error: %+v", closeErr)
		}
	}()

	log.Println(req.RemoteAddr, " ", resp.Status)

	removeHopHeaders(resp.Header)
	removeConnectionHeaders(resp.Header)

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, copyErr := io.Copy(w, resp.Body)
	if copyErr != nil {
		log.Printf("ServeHTTP copy body error: %+v", copyErr)
	}
}

func (p *forwardProxy) getHTTPClient() (*http.Client, error) {
	auth := proxy.Auth{
		User:     p.SocksUser,
		Password: p.SocksPassword,
	}

	dialer, err := proxy.SOCKS5("tcp", p.SocksServer, &auth, nil)
	if err != nil {
		return nil, err
	}

	contextDialer := dialer.(proxy.ContextDialer) //nolint:errcheck // definition of function before it called

	// Client request timeouts from cloudflare blog recommendations
	// https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext:           contextDialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}, nil
}

func (p *forwardProxy) proxyConnect(w http.ResponseWriter, req *http.Request) {
	log.Printf("CONNECT requested to %v (from %v)", req.Host, req.RemoteAddr)
	targetConn, err := net.Dial("tcp", req.Host)
	if err != nil {
		log.Println("failed to dial to target", req.Host)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Println("http server doesn't support hijacking connection")
		return
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Println("http hijacking failed")
		return
	}

	log.Println("tunnel established")
	go p.tunnelConn(targetConn, clientConn)
	go p.tunnelConn(clientConn, targetConn)
}

func (p *forwardProxy) tunnelConn(dst io.WriteCloser, src io.ReadCloser) {
	defer func() {
		err := dst.Close()
		if err != nil {
			log.Println("tunnel: failed close dst")
		}
	}()
	defer func() {
		err := src.Close()
		if err != nil {
			log.Println("tunnel: failed close src")
		}
	}()
	_, err := io.Copy(dst, src)
	if err != nil {
		log.Println("tunnel: failed copy")
	}
}

func main() {
	config, configErr := loadConfig()
	if configErr != nil {
		log.Fatal(configErr)
	}

	fp := &forwardProxy{
		SocksServer:   config.SocksProxy,
		SocksUser:     config.SocksProxyUser,
		SocksPassword: config.SocksProxyPassword,
	}

	log.Println("Starting proxy server on", config.HTTPAddress)
	if err := http.ListenAndServe(config.HTTPAddress, fp); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

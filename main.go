package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func main() {
	Listen()
}

// Listen starts an HTTP server on port 8080
func Listen() {
	port := 8080
	os.Getenv("PORT")
	if os.Getenv("PORT") == "" {
		slog.Info("No PORT environment variable set, using default port 8080")
	} else {
		port, _ = strconv.Atoi(os.Getenv("PORT"))
	}

	styxUrl := os.Getenv("STYX_URL")
	if styxUrl == "" {
		slog.Error("STYX_URL environment variable must be set")
		os.Exit(1)
	}

	slog.Info(fmt.Sprintf("Using STYX_URL: %s", styxUrl))

	clientCert := os.Getenv("CLIENT_CERT")
	if clientCert == "" {
		slog.Error("CLIENT_CERT environment variable must be set")
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		output := SendRequestToStyx(r, styxUrl, clientCert)
		defer output.Body.Close() // Ensure the response body is closed after copying

		for x, y := range output.Header {
			w.Header().Set(x, y[0]) // Set headers from the response to the response writer
		}
		w.WriteHeader(output.StatusCode) // Set the status code from the response

		written, err := io.Copy(w, output.Body)
		if err != nil {
			slog.Error("Failed to copy response body", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		slog.Info("Response body copied successfully", "bytes_written", written)
	})

	slog.Info(fmt.Sprintf("Starting HTTP server on :%d", port))
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux); err != nil {
		slog.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}

func SendRequestToStyx(inboundReq *http.Request, styxUrl string, clientCert string) *http.Response {
	url := styxUrl
	url += inboundReq.URL.Path
	slog.With("url", url).Info("URL set to")

	// Load client cert and key from combined file
	certFileContents := []byte(os.Getenv("CLIENT_CERT"))
	// Check if the certFile is empty
	if len(certFileContents) == 0 {
		slog.Error("Client certificate/key file is empty")
		os.Exit(1)
	}
	// Load the client certificate and key
	cert, err := tls.X509KeyPair(certFileContents, certFileContents)
	if err != nil {
		slog.Error("Failed to load client certificate/key from cert_and_key_for_pvault.crt", "error", err)
		os.Exit(1)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		slog.Info("Redirect detected", "url", req.URL.String(), "via", len(via))
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("Failed to create request", "error", err)
		os.Exit(1)
	}

	siteId := inboundReq.Header.Get("PContext-Site-ID")
	if siteId == "" {
		slog.Error("PContext-Site-ID header is missing")
		os.Exit(1)
	}

	envId := inboundReq.Header.Get("PContext-Site-Env")
	if envId == "" {
		slog.Error("PContext-Site-Env header is missing")
		os.Exit(1)
	}

	zone := inboundReq.Header.Get("PContext-Zone")
	if zone == "" {
		slog.Error("PContext-Zone header is missing")
		os.Exit(1)
	}

	req.Header.Set("PContext-Site-ID", siteId)
	req.Header.Set("PContext-Site-Env", envId)
	req.Header.Set("PContext-Zone", zone)
	req.Host = inboundReq.Host

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Request failed", "error", err)
		os.Exit(1)
	}
	if resp.TLS == nil {
		slog.Error("No TLS information in response")
	} else {
		slog.Info(resp.TLS.ServerName)
	}

	resp.Header.Set("X-styx-proxy", "true")

	surrogateKeys := resp.Header.Get("surrogate-key")
	if surrogateKeys != "" {
		strings.Replace(surrogateKeys, " ", ",", -1)
		resp.Header.Set("cache-tag", surrogateKeys)
		resp.Header.Del("surrogate-key")
	}

	slog.Info("Response status", "status", resp.Status)
	slog.Info("Response headers", "headers", resp.Header)
	return resp
}

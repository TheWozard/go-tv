package server

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"tailscale.com/tsnet"
)

// StartHTTP starts an HTTP server and blocks until ctx is cancelled, then
// shuts down gracefully.
func StartHTTP(ctx context.Context, handler http.Handler, port string) error {
	addr := fmt.Sprintf(":%s", port)
	srv := &http.Server{Addr: addr, Handler: handler}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	log.Printf("go-tv listening on http://localhost%s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// StartTailscale starts an HTTPS server over Tailscale and blocks until ctx
// is cancelled, then shuts down gracefully.
func StartTailscale(ctx context.Context, handler http.Handler, hostname, dir, port string) error {
	ts := &tsnet.Server{
		Dir:      dir,
		Hostname: hostname,
	}
	defer ts.Close()

	ln, err := ts.ListenTLS("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return fmt.Errorf("tailscale listen: %w", err)
	}

	lc, err := ts.LocalClient()
	if err == nil {
		if status, err := lc.Status(ctx); err == nil && status.Self != nil {
			log.Printf("go-tv listening on https://%s", status.Self.DNSName)
		}
	}
	if err != nil {
		log.Printf("go-tv listening on tailscale hostname %s", hostname)
	}

	srv := &http.Server{Handler: handler}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

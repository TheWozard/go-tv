package config

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"tailscale.com/tsnet"
)

type Tailscale struct {
	Hostname string `yaml:"hostname"`
	Dir      string `yaml:"dir"`
	Port     string `yaml:"port"`
}

func (t Tailscale) Enabled() bool { return t.Hostname != "" }

// Listen starts an HTTPS server over Tailscale and blocks until ctx
// is cancelled, then shuts down gracefully.
func (t Tailscale) Listen(ctx context.Context, handler http.Handler) error {
	ts := &tsnet.Server{
		Dir:      t.Dir,
		Hostname: t.Hostname,
	}
	defer ts.Close()

	ln, err := ts.ListenTLS("tcp", fmt.Sprintf(":%s", t.Port))
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
		log.Printf("go-tv listening on tailscale hostname %s", t.Hostname)
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

type Server struct {
	Port string `yaml:"port"`
}

// Listen starts an HTTP server and blocks until ctx is cancelled, then
// shuts down gracefully.
func (s Server) Listen(ctx context.Context, handler http.Handler) error {
	addr := fmt.Sprintf(":%s", s.Port)
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

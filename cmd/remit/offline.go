package main

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"github.com/transmit-bug/remit/internal/agentapi"
	"github.com/transmit-bug/remit/internal/core/store"
	"github.com/transmit-bug/remit/internal/core/transfer"
	"github.com/transmit-bug/remit/internal/settings"
)

// runOffline performs the two library commands that need no window, and
// reports whether it did one: exporting a library and unpacking an archive.
func runOffline(dir, exportPath, importPath string) bool {
	if exportPath == "" && importPath == "" {
		return false
	}

	if importPath != "" {
		src, err := os.Open(importPath)
		if err != nil {
			log.Fatalf("open %s: %v", importPath, err)
		}
		defer src.Close()
		info, err := src.Stat()
		if err != nil {
			log.Fatalf("stat %s: %v", importPath, err)
		}
		if err := transfer.Read(src, info.Size(), dir); err != nil {
			log.Fatalf("import: %v", err)
		}
		fmt.Printf("imported %s into %s\n", importPath, dir)
		return true
	}

	if _, err := os.Stat(filepath.Join(dir, "remit.json")); err != nil {
		log.Fatalf("no library at %s: %v", dir, err)
	}
	dst, err := os.Create(exportPath)
	if err != nil {
		log.Fatalf("create %s: %v", exportPath, err)
	}
	if err := transfer.Write(dir, dst); err != nil {
		dst.Close()
		log.Fatalf("export: %v", err)
	}
	if err := dst.Close(); err != nil {
		log.Fatalf("close %s: %v", exportPath, err)
	}
	fmt.Printf("exported %s to %s\n", dir, exportPath)
	return true
}

// startAgentServer serves the local agent API when the user enabled it. The
// port is chosen by the system and written beside the token, so a client finds
// both without a fixed port; nothing is served otherwise.
func startAgentServer(lib *store.Library) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		slog.Warn("remit: agent api: no data directory", "err", err)
		return
	}
	s, err := settings.Load(dir)
	if err != nil {
		slog.Warn("remit: agent api: settings", "err", err)
		return
	}
	if !s.AgentServer {
		return
	}
	token, err := agentapi.LoadOrCreateToken(filepath.Join(dir, "agent-token"))
	if err != nil {
		slog.Warn("remit: agent api: token", "err", err)
		return
	}
	ln, err := serveAgent(lib, token)
	if err != nil {
		slog.Warn("remit: agent api: listen", "err", err)
		return
	}
	addr := ln.Addr().String()
	if err := os.WriteFile(filepath.Join(dir, "agent-address"), []byte(addr+"\n"), 0o600); err != nil {
		slog.Warn("remit: agent api: address file", "err", err)
	}
	slog.Info("remit: agent api listening", "addr", addr)
}

// serveAgent serves the agent API on a loopback port the system chooses, and
// returns the listener. Loopback only: nothing on the network can reach it.
func serveAgent(lib *store.Library, token string) (net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	srv := agentapi.New(lib, agentapi.Options{Token: token})
	go func() {
		if err := srv.Serve(ln); err != nil {
			slog.Warn("remit: agent api stopped", "err", err)
		}
	}()
	return ln, nil
}

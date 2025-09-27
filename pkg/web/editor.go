package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"sort"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
	"nexus-sites.net/k3s-mgmt-tool/pkg/buildfile"
)

//go:embed static/* templates/*
var content embed.FS

// Server encapsulates the local web editor server
type Server struct {
	Addr     string // host:port; if empty, an available local port will be chosen
	httpSrv  *http.Server
	tmpl     *template.Template
}

// Start launches the web server. It blocks until the server is ready to accept connections, then returns the listening URL.
func (s *Server) Start() (string, error) {
	// Parse templates from embed
	tmpl, err := template.ParseFS(content, "templates/*.html")
	if err != nil {
		return "", fmt.Errorf("parse templates: %w", err)
	}
	s.tmpl = tmpl

	mux := http.NewServeMux()

	// Static assets (serve from embedded /static)
	sub, err := fs.Sub(content, "static")
	if err != nil {
		return "", fmt.Errorf("static fs: %w", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))

	// Routes
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/buildfile", s.handleBuildfile)
	// Interactive sections
	mux.HandleFunc("/api/sections/repos", s.handleReposGet)
	mux.HandleFunc("/api/sections/repos/add", s.handleReposAdd)
	mux.HandleFunc("/api/sections/repos/del", s.handleReposDel)
	mux.HandleFunc("/api/sections/charts", s.handleChartsGet)
	mux.HandleFunc("/api/sections/charts/add", s.handleChartsAdd)
	mux.HandleFunc("/api/sections/charts/del", s.handleChartsDel)
	mux.HandleFunc("/api/sections/manifests", s.handleManifestsGet)
	mux.HandleFunc("/api/sections/manifests/add", s.handleManifestsAdd)
	mux.HandleFunc("/api/sections/manifests/del", s.handleManifestsDel)
	mux.HandleFunc("/api/sections/scripts", s.handleScriptsGet)
	mux.HandleFunc("/api/sections/scripts/add", s.handleScriptsAdd)
	mux.HandleFunc("/api/sections/scripts/del", s.handleScriptsDel)
	mux.HandleFunc("/api/sections/gitrepos", s.handleGitReposGet)
	mux.HandleFunc("/api/sections/gitrepos/add", s.handleGitReposAdd)
	mux.HandleFunc("/api/sections/gitrepos/del", s.handleGitReposDel)

	// Listener
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}

	s.httpSrv = &http.Server{
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := s.httpSrv.Serve(l); err != nil && err != http.ErrServerClosed {
			log.Printf("web editor server error: %v", err)
		}
	}()

	url := "http://" + l.Addr().String() + "/"
	return url, nil
}

// WaitForInterrupt blocks until SIGINT/SIGTERM then gracefully shuts down the server.
func (s *Server) WaitForInterrupt() error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := map[string]any{
		"Title": "k3s-mgmt-tool: Buildfile Editor",
	}
	if err := s.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleBuildfile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Return YAML content as text/plain (loaded as-is)
		f, err := buildfile.LoadFromFile()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to load Buildfile.yaml: %v", err), http.StatusInternalServerError)
			return
		}
		// Ensure defaults, then render back to YAML
		f.EnsureInit()
		// Marshal to YAML
		data, err := yaml.Marshal(f)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to render YAML: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
	case http.MethodPost:
		// Accept posted textarea named "content"
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		body := r.PostFormValue("content")
		// Parse YAML into model to validate, then save
		var mf buildfile.SetupFile
		if err := yaml.Unmarshal([]byte(body), &mf); err != nil {
			http.Error(w, fmt.Sprintf("YAML parse error: %v", err), http.StatusBadRequest)
			return
		}
		mf.EnsureInit()
		if err := mf.SaveToFile(); err != nil {
			http.Error(w, fmt.Sprintf("failed to save Buildfile.yaml: %v", err), http.StatusInternalServerError)
			return
		}
		// Respond with a small HTMX snippet to show success toast
		w.Header().Set("HX-Trigger", "saved")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("OK"))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ---- Interactive section handlers ----

type kv struct{ Key, Value string }

type sectionData struct{ Items any }

func (s *Server) loadSetup() (*buildfile.SetupFile, error) {
	f, err := buildfile.LoadFromFile()
	if err != nil {
		return nil, err
	}
	f.EnsureInit()
	return f, nil
}

func (s *Server) renderSection(w http.ResponseWriter, name string, data sectionData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s error: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// Repos
func (s *Server) handleReposGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	items := make([]kv, 0, len(f.HelmRepos))
	for k, v := range f.HelmRepos { items = append(items, kv{k, v}) }
	sortSliceKV(items)
	s.renderSection(w, "repos", sectionData{Items: items})
}

func (s *Server) handleReposAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if err := r.ParseForm(); err != nil { http.Error(w, "bad form", http.StatusBadRequest); return }
	name := strings.TrimSpace(r.PostFormValue("name"))
	url := strings.TrimSpace(r.PostFormValue("url"))
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	f.AddHelmRepo(name, url)
	_ = f.SaveToFile()
	s.handleReposGet(w, r.Clone(r.Context()))
}

func (s *Server) handleReposDel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if err := r.ParseForm(); err != nil { http.Error(w, "bad form", http.StatusBadRequest); return }
	key := r.PostFormValue("key")
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	_ = f.RemoveHelmRepo(key)
	_ = f.SaveToFile()
	s.handleReposGet(w, r.Clone(r.Context()))
}

// Charts
func (s *Server) handleChartsGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	items := make([]kv, 0, len(f.HelmCharts))
	for k, v := range f.HelmCharts { items = append(items, kv{k, v}) }
	sortSliceKV(items)
	s.renderSection(w, "charts", sectionData{Items: items})
}

func (s *Server) handleChartsAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if err := r.ParseForm(); err != nil { http.Error(w, "bad form", http.StatusBadRequest); return }
	release := strings.TrimSpace(r.PostFormValue("release"))
	chart := strings.TrimSpace(r.PostFormValue("chart"))
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	_ = f.AddHelmChart(release, chart)
	_ = f.SaveToFile()
	s.handleChartsGet(w, r.Clone(r.Context()))
}

func (s *Server) handleChartsDel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if err := r.ParseForm(); err != nil { http.Error(w, "bad form", http.StatusBadRequest); return }
	key := r.PostFormValue("key")
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	_ = f.RemoveHelmChart(key)
	_ = f.SaveToFile()
	s.handleChartsGet(w, r.Clone(r.Context()))
}

// Manifests
func (s *Server) handleManifestsGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	items := append([]string(nil), f.Manifests...)
	s.renderSection(w, "manifests", sectionData{Items: items})
}

func (s *Server) handleManifestsAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if err := r.ParseForm(); err != nil { http.Error(w, "bad form", http.StatusBadRequest); return }
	val := strings.TrimSpace(r.PostFormValue("value"))
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	if val != "" { f.Manifests = append(f.Manifests, val) }
	_ = f.SaveToFile()
	s.handleManifestsGet(w, r.Clone(r.Context()))
}

func (s *Server) handleManifestsDel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if err := r.ParseForm(); err != nil { http.Error(w, "bad form", http.StatusBadRequest); return }
	val := r.PostFormValue("value")
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	filtered := make([]string, 0, len(f.Manifests))
	for _, it := range f.Manifests { if it != val { filtered = append(filtered, it) } }
	f.Manifests = filtered
	_ = f.SaveToFile()
	s.handleManifestsGet(w, r.Clone(r.Context()))
}

// Scripts
func (s *Server) handleScriptsGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	items := append([]string(nil), f.StartupScripts...)
	s.renderSection(w, "scripts", sectionData{Items: items})
}

func (s *Server) handleScriptsAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if err := r.ParseForm(); err != nil { http.Error(w, "bad form", http.StatusBadRequest); return }
	val := strings.TrimSpace(r.PostFormValue("value"))
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	if val != "" { f.StartupScripts = append(f.StartupScripts, val) }
	_ = f.SaveToFile()
	s.handleScriptsGet(w, r.Clone(r.Context()))
}

func (s *Server) handleScriptsDel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if err := r.ParseForm(); err != nil { http.Error(w, "bad form", http.StatusBadRequest); return }
	val := r.PostFormValue("value")
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	filtered := make([]string, 0, len(f.StartupScripts))
	for _, it := range f.StartupScripts { if it != val { filtered = append(filtered, it) } }
	f.StartupScripts = filtered
	_ = f.SaveToFile()
	s.handleScriptsGet(w, r.Clone(r.Context()))
}

// Git Repos
func (s *Server) handleGitReposGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	items := make([]kv, 0, len(f.GitRepos))
	for k, v := range f.GitRepos { items = append(items, kv{k, v}) }
	sortSliceKV(items)
	s.renderSection(w, "gitrepos", sectionData{Items: items})
}

func (s *Server) handleGitReposAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if err := r.ParseForm(); err != nil { http.Error(w, "bad form", http.StatusBadRequest); return }
	name := strings.TrimSpace(r.PostFormValue("name"))
	url := strings.TrimSpace(r.PostFormValue("url"))
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	if name != "" && url != "" { f.GitRepos[name] = url }
	_ = f.SaveToFile()
	s.handleGitReposGet(w, r.Clone(r.Context()))
}

func (s *Server) handleGitReposDel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if err := r.ParseForm(); err != nil { http.Error(w, "bad form", http.StatusBadRequest); return }
	key := r.PostFormValue("key")
	f, err := s.loadSetup(); if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	delete(f.GitRepos, key)
	_ = f.SaveToFile()
	s.handleGitReposGet(w, r.Clone(r.Context()))
}

func sortSliceKV(items []kv) {
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, path.Clean(r.URL.Path), time.Since(start))
	})
}

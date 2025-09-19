package websvc

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

type Config struct {
	Enable bool
	Addr   string
	Root   string
	Proxy  ProxyConfig
}

type ProxyConfig struct {
	Enable bool
	Target string
	Mount  string
}

type Server struct {
	config     *Config
	httpServer *http.Server
	proxy      *httputil.ReverseProxy
}

func NewServer(config *Config) *Server {
	s := &Server{
		config: config,
	}

	if config.Proxy.Enable && config.Proxy.Target != "" {
		target, err := url.Parse(config.Proxy.Target)
		if err == nil {
			s.proxy = httputil.NewSingleHostReverseProxy(target)
			s.proxy.ModifyResponse = s.modifyProxyResponse
		}
	}

	return s
}

func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Static file serving
	if s.config.Root != "" {
		router.Static("/static", s.config.Root)
		router.StaticFile("/", filepath.Join(s.config.Root, "index.html"))
	}

	// Proxy routes
	if s.proxy != nil && s.config.Proxy.Mount != "" {
		proxyGroup := router.Group(s.config.Proxy.Mount)
		proxyGroup.Any("/*path", s.handleProxy)
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	s.httpServer = &http.Server{
		Addr:    s.config.Addr,
		Handler: router,
	}

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Web server error: %v\n", err)
		}
	}()

	fmt.Printf("Web server started on %s\n", s.config.Addr)
	return nil
}

func (s *Server) Stop() error {
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

func (s *Server) ServePath(path string) {
	s.config.Root = path
}

func (s *Server) GetURL() string {
	return fmt.Sprintf("http://%s", s.config.Addr)
}

func (s *Server) handleProxy(c *gin.Context) {
	if s.proxy == nil {
		c.JSON(503, gin.H{"error": "proxy not configured"})
		return
	}

	// Remove the mount prefix from the path
	path := c.Param("path")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Modify the request URL to point to the target
	originalPath := c.Request.URL.Path
	c.Request.URL.Path = path

	// Set headers for the proxy
	c.Request.Header.Set("X-Forwarded-For", c.ClientIP())
	c.Request.Header.Set("X-Forwarded-Proto", "http")
	c.Request.Header.Set("X-Forwarded-Host", c.Request.Host)

	// Use the reverse proxy
	s.proxy.ServeHTTP(c.Writer, c.Request)

	// Restore original path
	c.Request.URL.Path = originalPath
}

func (s *Server) modifyProxyResponse(resp *http.Response) error {
	// Remove headers that prevent iframe embedding
	resp.Header.Del("X-Frame-Options")
	resp.Header.Del("Content-Security-Policy")
	resp.Header.Del("X-Content-Type-Options")

	// Optionally modify CSP to allow embedding
	if csp := resp.Header.Get("Content-Security-Policy"); csp != "" {
		// Remove frame-ancestors directive or modify it
		newCSP := strings.ReplaceAll(csp, "frame-ancestors 'none'", "frame-ancestors *")
		newCSP = strings.ReplaceAll(newCSP, "frame-ancestors 'self'", "frame-ancestors *")
		resp.Header.Set("Content-Security-Policy", newCSP)
	}

	return nil
}

// ProxyReader wraps the response body to modify content if needed
type ProxyReader struct {
	reader io.Reader
}

func (pr *ProxyReader) Read(p []byte) (n int, err error) {
	return pr.reader.Read(p)
}
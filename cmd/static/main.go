package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	apiURL, _ := url.Parse("http://localhost:8080")
	proxy := httputil.NewSingleHostReverseProxy(apiURL)

	r.Any("/api/*path", func(c *gin.Context) {
		c.Request.URL.Path = "/api/" + c.Param("path")
		proxy.ServeHTTP(c.Writer, c.Request)
	})

	r.GET("/ws", func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	})

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("fail on get current work dir: %v", err)
	}

	static := filepath.Join(cwd, "dist")
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		full := filepath.Join(static, path)
		if _, err := os.Stat(full); err == nil && !strings.HasSuffix(path, "/") {
			c.File(full)
			return
		}
		c.File(filepath.Join(static, "index.html"))
	})

	if err := r.Run(":9000"); err != nil {
		log.Fatal(err)
	}
}

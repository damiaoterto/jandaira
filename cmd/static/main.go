package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("fail on get current work dir: %v", err)
	}

	static := filepath.Join(cwd, "dist")

	r.StaticFS("/", gin.Dir(static, false))

	if err := r.Run(":9000"); err != nil {
		log.Fatal(err)
	}
}

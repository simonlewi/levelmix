package main

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// Get directory of current file
	_, b, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(b), "..")

	templates := []string{
		filepath.Join(projectRoot, "templates", "pages", "home.html"),
		filepath.Join(projectRoot, "templates", "pages", "process.html"),
	}
	r.SetHTMLTemplate(template.Must(template.ParseFiles(templates...)))

	// Serve static files
	r.Static("/static", filepath.Join(projectRoot, "static"))

	// Routes
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "home.html", gin.H{})
	})

	r.GET("/process", func(c *gin.Context) {
		c.HTML(http.StatusOK, "process.html", gin.H{})
	})

	// Start server
	log.Println("Server starting on http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

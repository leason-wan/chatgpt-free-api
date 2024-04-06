package main

import (
	"bufio"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"

	"aurora/internal/proxys"

	"github.com/acheong08/endless"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

//go:embed web/*
var staticFiles embed.FS

func checkProxy() *proxys.IProxy {
	var proxies []string
	proxyUrl := os.Getenv("PROXY_URL")
	if proxyUrl != "" {
		proxies = append(proxies, proxyUrl)
	}

	if _, err := os.Stat("proxies.txt"); err == nil {
		file, err := os.Open("proxies.txt")
		if err != nil {
			logrus.Warn("Failed to open proxies.txt", "err", err)
			return nil // or handle error more gracefully
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			proxy := scanner.Text()
			parsedURL, err := url.Parse(proxy)
			if err != nil {
				logrus.Warn("proxy url is invalid", "url", proxy, "err", err)
				continue
			}

			if parsedURL.Port() != "" {
				proxies = append(proxies, proxy)
			} else {
				continue
			}
		}
	}

	proxyIP := proxys.NewIProxyIP(proxies)
	return &proxyIP
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func main() {
	gin.SetMode(gin.ReleaseMode)

	_ = godotenv.Load(".env")
	host := os.Getenv("SERVER_HOST")
	port := os.Getenv("SERVER_PORT")
	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")

	router := gin.Default()
	router.Use(corsMiddleware())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Hello, world!"})
	})
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	router.POST("/auth/session", sessionHandler)
	router.POST("/auth/refresh", refreshHandler)
	router.OPTIONS("/v1/chat/completions", optionsHandler)

	authGroup := router.Group("").Use(authorizationMiddleware())
	authGroup.POST("/v1/chat/completions", func(c *gin.Context) {
		// Implement your chat completions logic here
		c.JSON(http.StatusOK, gin.H{"message": "Chat completion"})
	})
	authGroup.GET("/v1/models", func(c *gin.Context) {
		// Implement your models listing logic here
		c.JSON(http.StatusOK, gin.H{"message": "List of models"})
	})

	subFS, err := fs.Sub(staticFiles, "web")
	if err != nil {
		log.Fatal(err)
	}
	router.StaticFS("/web", http.FS(subFS))

	if host == "" {
		host = "0.0.0.0"
	}
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on %s:%s\n", host, port)
	if tlsCert != "" && tlsKey != "" {
		_ = endless.ListenAndServeTLS(host+":"+port, tlsCert, tlsKey, router)
	} else {
		_ = endless.ListenAndServe(host+":"+port, router)
	}
}

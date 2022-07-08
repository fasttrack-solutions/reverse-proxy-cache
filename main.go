package main

import (
	"log"
	"net/http"
	"os"

	"github.com/GitbookIO/diskache"
	"github.com/fasttrack-solutions/reverse-proxy-cache/reverseproxy"
	"github.com/gin-gonic/gin"
)

func main() {
	apiKey := os.Getenv("API_KEY")
	target := os.Getenv("TARGET")
	bearerToken := os.Getenv("TARGET_BEARER_TOKEN")
	port := os.Getenv("PORT")
	removeFromPath := os.Getenv("REMOVE_FROM_PATH")
	useRawURLForGin := os.Getenv("USE_RAW_URL_FOR_GIN")

	// Create an instance
	opts := diskache.Opts{
		Directory: "diskcache_place",
	}
	dc, err := diskache.New(&opts)

	if err != nil {
		panic(err)
	}

	proxy := reverseproxy.New(target, bearerToken, dc, removeFromPath)

	router := gin.Default()

	if useRawURLForGin == "true" {
		router.UseRawPath = true
		router.UnescapePathValues = false
	}

	router.Use(Auth(apiKey))
	router.POST("/cache/clear", func(c *gin.Context) {
		err := dc.Clean()

		if err != nil {
			c.AbortWithError(500, err)
		}

		c.Status(200)
	})
	router.GET("proxy/*url", gin.WrapF(proxy.HandleRequest))
	router.Run(":" + port)

	// start server
	// http.HandleFunc("/proxy", proxy.HandleRequest)
	// if err := http.ListenAndServe(":"+port, nil); err != nil {
	// 	panic(err)
	// }
	log.Println("Started on " + port)

}

func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("x-api-key")

		if secret == "" {
			log.Print("No api key set")
			c.Next()
			return
		}

		if secret != apiKey {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}

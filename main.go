package main

import (
	"log"
	"os"

	"github.com/GitbookIO/diskache"
	"github.com/fasttrack-solutions/reverse-proxy-cache/reverseproxy"
	"github.com/gin-gonic/gin"
)

func main() {
	target := os.Getenv("TARGET")
	bearerToken := os.Getenv("BEARER_TOKEN")
	port := os.Getenv("PORT")

	// Create an instance
	opts := diskache.Opts{
		Directory: "diskache_place",
	}
	dc, err := diskache.New(&opts)

	if err != nil {
		panic(err)
	}

	proxy := reverseproxy.New(target, bearerToken, dc)

	router := gin.Default()
	router.POST("/cache/clear", func(c *gin.Context) {
		err := dc.Clean()

		if err != nil {
			c.AbortWithError(500, err)
		}

		c.Status(200)
	})
	router.Any("proxy/*url", gin.WrapF(proxy.HandleRequest))
	router.Run(":" + port)

	// start server
	// http.HandleFunc("/proxy", proxy.HandleRequest)
	// if err := http.ListenAndServe(":"+port, nil); err != nil {
	// 	panic(err)
	// }
	log.Println("Started on " + port)

}

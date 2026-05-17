package api

import (
	"fmt"
	"net"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/miekg/dns"

	c "github.com/ray-g/dnsproxy/cache"
	"github.com/ray-g/dnsproxy/logger"
	"github.com/ray-g/dnsproxy/stats"
)

func StartAPIServer(addr string, debugMode bool, cache c.Cache) error {
	router := newRouter(debugMode)
	router.Use(cors.Default())

	registerRoutes(router, cache)

	if debugMode {
		router.Use(static.Serve("/", static.LocalFile("./web", false)))
	} else {
		router.Use(static.Serve("/", BinaryFileSystem("")))
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	server := &http.Server{Addr: addr, Handler: router}
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			logger.Fatal(err)
		}
	}()

	logger.Infof("API server listening on %s", addr)
	return nil
}

func newRouter(debugMode bool) *gin.Engine {
	if !debugMode {
		gin.SetMode(gin.ReleaseMode)
		r := gin.New()
		r.Use(gin.Recovery())
		return r
	}
	return gin.Default()
}

func registerRoutes(router *gin.Engine, cache c.Cache) {
	router.GET("/cache", handleCacheDump(cache))
	router.GET("/cache/length", handleCacheLength(cache))
	router.GET("/cache/:key", handleCacheGet(cache))
	router.DELETE("/cache/:key", handleCacheDelete(cache))
	router.GET("/query/:key", handleQuery(cache))
	router.GET("/stats", handleStats())
	router.GET("/application/active", handleGetActive())
	router.PUT("/application/active", handleSetActive())
}

func handleCacheDump(cache c.Cache) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"cache": cache.Dump()})
	}
}

func handleCacheLength(cache c.Cache) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"length": cache.Length()})
	}
}

func handleCacheGet(cache c.Cache) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		key := ctx.Param("key")
		r, err := cache.Get(key)
		if err != nil {
			ctx.JSON(http.StatusOK, gin.H{"error": key + " not found"})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"answer": r.Msg.Answer})
	}
}

func handleCacheDelete(cache c.Cache) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		key := ctx.Param("key")
		cache.Remove(key)
		ctx.JSON(http.StatusOK, gin.H{"key": key})
	}
}

func handleQuery(cache c.Cache) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		key := ctx.Param("key")
		cr, ce := cache.Get(key)

		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(key), dns.TypeA)
		r, _, e := new(dns.Client).Exchange(m, "127.0.0.1:53")

		resp := gin.H{}
		if e != nil || r == nil || r.Rcode != dns.RcodeSuccess {
			resp["query"] = fmt.Sprintf("failed to resolve %s", key)
		} else {
			resp["query"] = r.Answer
		}

		if ce != nil {
			resp["cache"] = fmt.Sprintf("%s not in cache", key)
		} else {
			resp["cache"] = cr.Msg.Answer
		}

		ctx.JSON(http.StatusOK, resp)
	}
}

func handleStats() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"stats": stats.Dump()})
	}
}

func handleGetActive() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"active": stats.Active()})
	}
}

func handleSetActive() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Query("v") != "1" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Illegal value for 'version'"})
			return
		}
		switch ctx.Query("state") {
		case "On":
			stats.Activate()
			ctx.JSON(http.StatusOK, gin.H{"active": stats.Active()})
		case "Off":
			stats.Deactivate()
			ctx.JSON(http.StatusOK, gin.H{"active": stats.Active()})
		default:
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Illegal value for 'state'"})
		}
	}
}

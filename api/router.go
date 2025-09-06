package api

import (
	"github.com/gin-gonic/gin"
	"pansou/config"
	"pansou/service"
	"pansou/util"
)
import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// ✅ 1. 打包 dist 目录
//
//go:embed dist/*
var embedDist embed.FS

// SetupRouter 设置路由
func SetupRouter(searchService *service.SearchService) *gin.Engine {
	// 设置搜索服务
	SetSearchService(searchService)

	// 设置为生产模式
	gin.SetMode(gin.ReleaseMode)

	// 创建默认路由
	r := gin.Default()

	// 添加中间件
	r.Use(CORSMiddleware())
	r.Use(LoggerMiddleware())
	r.Use(util.GzipMiddleware()) // 添加压缩中间件

	// 定义API路由组
	api := r.Group("/api")
	{
		// 搜索接口 - 支持POST和GET两种方式
		api.POST("/search", SearchHandler)
		api.GET("/search", SearchHandler) // 添加GET方式支持

		// 健康检查接口
		api.GET("/health", func(c *gin.Context) {
			// 根据配置决定是否返回插件信息
			pluginCount := 0
			pluginNames := []string{}
			pluginsEnabled := config.AppConfig.AsyncPluginEnabled

			if pluginsEnabled && searchService != nil && searchService.GetPluginManager() != nil {
				plugins := searchService.GetPluginManager().GetPlugins()
				pluginCount = len(plugins)
				for _, p := range plugins {
					pluginNames = append(pluginNames, p.Name())
				}
			}

			// 获取频道信息
			channels := config.AppConfig.DefaultChannels
			channelsCount := len(channels)

			response := gin.H{
				"status":          "ok",
				"plugins_enabled": pluginsEnabled,
				"channels":        channels,
				"channels_count":  channelsCount,
			}

			// 只有当插件启用时才返回插件相关信息
			if pluginsEnabled {
				response["plugin_count"] = pluginCount
				response["plugins"] = pluginNames
			}

			c.JSON(200, response)
		})
	}

	// ✅ 2. 子目录转换 (frontend/dist -> distFS)
	distFS, err := fs.Sub(embedDist, "dist")
	if err != nil {
		panic(err)
	}

	// ✅ 所有非 API 的请求都走这里
	r.NoRoute(func(c *gin.Context) {
		reqPath := strings.TrimPrefix(c.Request.URL.Path, "/")
		if reqPath == "" {
			reqPath = "index.html"
		}

		// 优先查找静态文件
		file, err := distFS.Open(reqPath)
		if err != nil {
			// 如果文件不存在 → 返回 index.html (Vue Router fallback)
			file, err = distFS.Open("index.html")
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			defer file.Close()
			stat, _ := file.Stat()
			c.DataFromReader(http.StatusOK, stat.Size(), "text/html", file, nil)
			return
		}

		// 如果文件存在 → 直接返回该文件
		defer file.Close()
		stat, _ := file.Stat()
		// todo 这里需要根据文件类型,自动设置
		c.DataFromReader(http.StatusOK, stat.Size(), mime.TypeByExtension(filepath.Ext(reqPath)), file, nil)
	})

	return r
}

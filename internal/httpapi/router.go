package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/x-dora/rw-node-go/internal/config"
)

type Handlers struct {
	Xray     XrayHandlers
	Handler  HandlerHandlers
	Stats    StatsHandlers
	Vision   VisionHandlers
	Plugin   PluginHandlers
	Internal InternalHandlers
}

type XrayHandlers interface {
	Start(*gin.Context)
	Stop(*gin.Context)
	Healthcheck(*gin.Context)
}

type HandlerHandlers interface {
	AddUser(*gin.Context)
	AddUsers(*gin.Context)
	RemoveUser(*gin.Context)
	RemoveUsers(*gin.Context)
	GetInboundUsers(*gin.Context)
	GetInboundUsersCount(*gin.Context)
	DropUsersConnections(*gin.Context)
	DropIPs(*gin.Context)
}

type StatsHandlers interface {
	GetSystemStats(*gin.Context)
	GetUsersStats(*gin.Context)
	GetUserOnlineStatus(*gin.Context)
	GetUserIPList(*gin.Context)
	GetUsersIPList(*gin.Context)
	GetInboundStats(*gin.Context)
	GetOutboundStats(*gin.Context)
	GetAllInboundsStats(*gin.Context)
	GetAllOutboundsStats(*gin.Context)
	GetCombinedStats(*gin.Context)
}

type VisionHandlers interface {
	BlockIP(*gin.Context)
	UnblockIP(*gin.Context)
}

type PluginHandlers interface {
	Sync(*gin.Context)
	CollectTorrentBlockerReports(*gin.Context)
	BlockIPs(*gin.Context)
	UnblockIPs(*gin.Context)
	RecreateTables(*gin.Context)
}

type InternalHandlers interface {
	GetConfig(*gin.Context)
	Webhook(*gin.Context)
}

func NewRouter(cfg config.Config, handlers Handlers, logger *slog.Logger) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(ginRecovery(logger), ginBodyLimit(cfg.RequestBodyLimitBytes))
	registerRoutes(router, handlers)

	return router
}

func registerRoutes(router gin.IRoutes, handlers Handlers) {
	router.POST("/node/xray/start", handlers.Xray.Start)
	router.GET("/node/xray/stop", handlers.Xray.Stop)
	router.GET("/node/xray/healthcheck", handlers.Xray.Healthcheck)

	router.POST("/node/handler/add-user", handlers.Handler.AddUser)
	router.POST("/node/handler/add-users", handlers.Handler.AddUsers)
	router.POST("/node/handler/remove-user", handlers.Handler.RemoveUser)
	router.POST("/node/handler/remove-users", handlers.Handler.RemoveUsers)
	router.POST("/node/handler/get-inbound-users", handlers.Handler.GetInboundUsers)
	router.POST("/node/handler/get-inbound-users-count", handlers.Handler.GetInboundUsersCount)
	router.POST("/node/handler/drop-users-connections", handlers.Handler.DropUsersConnections)
	router.POST("/node/handler/drop-ips", handlers.Handler.DropIPs)

	router.GET("/node/stats/get-system-stats", handlers.Stats.GetSystemStats)
	router.POST("/node/stats/get-users-stats", handlers.Stats.GetUsersStats)
	router.POST("/node/stats/get-user-online-status", handlers.Stats.GetUserOnlineStatus)
	router.POST("/node/stats/get-user-ip-list", handlers.Stats.GetUserIPList)
	router.GET("/node/stats/get-users-ip-list", handlers.Stats.GetUsersIPList)
	router.POST("/node/stats/get-inbound-stats", handlers.Stats.GetInboundStats)
	router.POST("/node/stats/get-outbound-stats", handlers.Stats.GetOutboundStats)
	router.POST("/node/stats/get-all-inbounds-stats", handlers.Stats.GetAllInboundsStats)
	router.POST("/node/stats/get-all-outbounds-stats", handlers.Stats.GetAllOutboundsStats)
	router.POST("/node/stats/get-combined-stats", handlers.Stats.GetCombinedStats)

	router.POST("/vision/block-ip", handlers.Vision.BlockIP)
	router.POST("/vision/unblock-ip", handlers.Vision.UnblockIP)

	router.POST("/node/plugin/sync", handlers.Plugin.Sync)
	router.POST("/node/plugin/torrent-blocker/collect", handlers.Plugin.CollectTorrentBlockerReports)
	router.POST("/node/plugin/nftables/block-ips", handlers.Plugin.BlockIPs)
	router.POST("/node/plugin/nftables/unblock-ips", handlers.Plugin.UnblockIPs)
	router.POST("/node/plugin/nftables/recreate-tables", handlers.Plugin.RecreateTables)

	router.GET("/internal/get-config", handlers.Internal.GetConfig)
	router.POST("/internal/webhook", handlers.Internal.Webhook)
}

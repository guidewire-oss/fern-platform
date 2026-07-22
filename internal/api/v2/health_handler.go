package v2

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Pinger is the readiness check's downstream dependency. The
// production wiring passes a thin wrapper around `sql.DB.PingContext`
// so the readiness probe surfaces real DB outages.
type Pinger interface {
	Ping(ctx context.Context) error
}

// RegisterHealthRoutes mounts /healthz and /readyz on r.
//
//   - /healthz returns 200 always (liveness probe). The process is
//     alive if Gin can serve a request; k8s uses this to decide
//     whether to restart the container.
//
//   - /readyz pings the supplied dependency and returns 503 if it
//     fails. k8s uses this to decide whether to route traffic.
//
// We do not collapse the two endpoints because k8s expects different
// semantics: a flapping DB should drain traffic from the pod (readiness
// fails) but not restart it (liveness stays green).
func RegisterHealthRoutes(r gin.IRouter, p Pinger) {
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/readyz", func(c *gin.Context) {
		if p == nil {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "skipped"})
			return
		}
		if err := p.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

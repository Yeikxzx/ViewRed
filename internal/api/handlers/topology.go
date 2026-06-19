package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/viewred/viewred/internal/storage"
)

// TopologyResponse represents the nodes and edges for the frontend React Flow.
type TopologyResponse struct {
	Nodes []storage.Device       `json:"nodes"`
	Edges []storage.TopologyEdge `json:"edges"`
}

// TopologyGet returns the network topology graph.
func TopologyGet(q storage.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		devices, edges, err := q.QueryTopology(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}

		if devices == nil {
			devices = []storage.Device{}
		}
		if edges == nil {
			edges = []storage.TopologyEdge{}
		}

		c.JSON(http.StatusOK, TopologyResponse{
			Nodes: devices,
			Edges: edges,
		})
	}
}

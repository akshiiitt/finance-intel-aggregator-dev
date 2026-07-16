package handlers

import (
	"github.com/financeintel/backend/internal/broker"
	"github.com/gin-gonic/gin"
)

// StreamSSE is the Gin handler for the SSE endpoint.
func StreamSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	clientChan := make(chan []byte, 10)
	broker.GlobalSSEBroker.AddClient(clientChan)
	defer broker.GlobalSSEBroker.RemoveClient(clientChan)

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-clientChan:
			c.SSEvent("message", string(msg))
			c.Writer.Flush()
		}
	}
}

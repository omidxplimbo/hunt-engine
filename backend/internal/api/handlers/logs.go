package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gofiber/contrib/websocket"
)

// StreamSystemLogs handles the WebSocket connection for system logs
func StreamSystemLogs(c *websocket.Conn) {
	// Ensure only admin
	// Note: Authentication is handled by middleware, but we can double check role from Locals
	// Locals are passed from the HTTP request context to the WS connection by Fiber
	role := c.Locals("role")
	// Type assertion might be needed, or string conversion
	roleStr, ok := role.(string)
	if !ok || roleStr != "admin" {
		c.WriteMessage(websocket.TextMessage, []byte("Access Denied: Admins only"))
		c.Close()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Docker Client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error creating docker client: %v", err)))
		return
	}
	defer cli.Close()

	// List containers
	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error listing containers: %v", err)))
		return
	}

	var wg sync.WaitGroup
	msgChan := make(chan string)

	// Helper to stream logs from one container
	streamContainer := func(id, name string) {
		defer wg.Done()

		// Clean name (remove leading /)
		name = strings.TrimPrefix(name, "/")

		options := container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Tail:       "50", // Fetch last 50 lines initially
			Timestamps: false,
		}

		reader, err := cli.ContainerLogs(ctx, id, options)
		if err != nil {
			msgChan <- fmt.Sprintf("Error getting logs for %s: %v", name, err)
			return
		}
		defer reader.Close()

		// Docker logs format: 8 byte header [STREAM_TYPE, 0, 0, 0, SIZE1, SIZE2, SIZE3, SIZE4]
		// followed by payload.
		hdr := make([]byte, 8)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Read Header
				_, err := io.ReadFull(reader, hdr)
				if err != nil {
					if err == io.EOF {
						return
					}
					log.Printf("Error reading log header for %s: %v\n", name, err)
					msgChan <- fmt.Sprintf("Error reading log header for %s: %v", name, err)
					return
				}

				// Extract payload size
				size := uint32(hdr[4])<<24 | uint32(hdr[5])<<16 | uint32(hdr[6])<<8 | uint32(hdr[7])

				if size > 0 {
					buf := make([]byte, size)
					_, err = io.ReadFull(reader, buf)
					if err != nil {
						log.Printf("Error reading log payload for %s: %v\n", name, err)
						msgChan <- fmt.Sprintf("Error reading log payload for %s: %v", name, err)
						return
					}

					line := string(buf)
					// Determine color based on container name
					color := "\x1b[36m" // Cyan default
					if strings.Contains(name, "backend") {
						color = "\x1b[32m" // Green
					} else if strings.Contains(name, "worker") {
						color = "\x1b[33m" // Yellow
					} else if strings.Contains(name, "postgres") {
						color = "\x1b[34m" // Blue
					} else if strings.Contains(name, "redis") {
						color = "\x1b[31m" // Red
					}

					// Send formatted log line
					// Format: [CONTAINER_NAME] LOG_CONTENT
					msgChan <- fmt.Sprintf("%s[%s]\x1b[0m %s", color, name, strings.TrimSpace(line))
				}
			}
		}
	}

	count := 0
	for _, cont := range containers {
		// Filter for Hunt Engine containers
		// Our docker-compose uses container_names like: hunt-backend, hunt-frontend, hunt-worker...
		isTarget := false
		for _, n := range cont.Names {
			if strings.Contains(n, "hunt-") {
				isTarget = true
				break
			}
		}

		if isTarget {
			wg.Add(1)
			go streamContainer(cont.ID, cont.Names[0])
			count++
		}
	}

	if count == 0 {
		c.WriteMessage(websocket.TextMessage, []byte("No 'hunt-' containers found. Is the stack running?"))
	}

	// Sender Loop: Reads from channel and writes to WebSocket
	go func() {
		for {
			select {
			case msg := <-msgChan:
				if err := c.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					cancel() // Stop all streams if client disconnects
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Reader Loop: Required to handle control frames (Ping/Pong/Close)
	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			break
		}
	}

	// Cleanup
	cancel()
	wg.Wait()
}

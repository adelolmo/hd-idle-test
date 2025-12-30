package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/madflojo/tasks"
)

const (
	socketFile = "/tmp/demo.sock"
)

func main() {
	router := gin.Default()

	scheduler := tasks.New()
	defer scheduler.Stop()

	configDir, _ := os.UserConfigDir()

	dataDir := filepath.Join(configDir, "hdtd")
	err := os.MkdirAll(dataDir, 0750)
	if err != nil {
		panic(err)
	}

	router.GET("/sessions", func(c *gin.Context) {
		type Response struct {
			Sessions []string `json:"sessions"`
		}

		var sessionDirs []string
		sessionDirNames, err := os.ReadDir(dataDir)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}

		for i := range sessionDirNames {
			sessionDirs = append(sessionDirs, sessionDirNames[i].Name())
		}

		c.JSON(http.StatusOK, Response{Sessions: sessionDirs})
	})

	router.GET("/sessions/:id", func(c *gin.Context) {
		sessionDir := filepath.Join(dataDir, c.Param("id"))

		type Response struct {
			Diskstats []string `json:"diskstats"`
		}

		diskstatsDir := filepath.Join(sessionDir, "diskstats")
		diskStatsFiles, err := os.ReadDir(diskstatsDir)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var diskstats []string
		for _, e := range diskStatsFiles {
			if e.IsDir() {
				continue
			}
			bytes, err := os.ReadFile(filepath.Join(diskstatsDir, e.Name()))
			if err != nil {
				log.Println(err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			diskstats = append(diskstats, string(bytes))
		}

		c.JSON(http.StatusOK, Response{Diskstats: diskstats})
	})

	router.POST("/record", func(c *gin.Context) {
		type Request struct {
			Action string `json:"action"`
		}
		var request Request
		err := c.ShouldBind(&request)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if request.Action == "start" {
			sessionDir := filepath.Join(dataDir, fmt.Sprintf("%d", time.Now().Unix()))
			err := os.MkdirAll(filepath.Join(sessionDir, "diskstats"), 0750)
			if err != nil {
				panic(err)
			}
			_, err = scheduler.Add(&tasks.Task{
				Interval: 1 * time.Second,
				TaskFunc: func() error {
					collectStats(sessionDir)
					return nil
				},
			})
			if err != nil {
				panic(err)
			}
			log.Printf("Starting recording...")
		}
		if request.Action == "stop" {
			scheduler.Stop()
			log.Printf("Stopping recording...")
		}

		c.Status(http.StatusOK)
	})

	err = os.Remove(socketFile)
	if err != nil {
		panic(err)
	}
	listener, err := net.Listen("unix", socketFile)
	if err != nil {
		panic(err)
	}

	err = http.Serve(listener, router)
	if err != nil {
		panic(err)
	}

}

func collectStats(dir string) {
	fmt.Println("Working...")
	bytesRead, err := os.ReadFile("/proc/diskstats")

	if err != nil {
		log.Fatal(err)
	}

	dest := filepath.Join(filepath.Join(dir, "diskstats"), fmt.Sprintf("%d", time.Now().Unix()))

	err = os.WriteFile(dest, bytesRead, 0644)

	if err != nil {
		log.Fatal(err)
	}
}

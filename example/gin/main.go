package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/jssyjam/ratelimit/middleware"
)

func main() {
	opt := &redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	}
	client := redis.NewClient(opt)
	gm, err := middleware.NewGinMiddleware(client, nil)
	if err != nil {
		panic(err)
	}
	r := gin.Default()
	r.Use(gm.RateLimit())
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{ // 返回一个JSON，状态码是200，gin.H是map[string]interface{}的简写
			"message": "pong",
		})
	})
	r.Run() // 启动服务，并默认监听8080端口
}

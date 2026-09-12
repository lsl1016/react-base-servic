package helpers

import (
	"context"
	"errors"

	"react-base-service/conf"
	"react-base-service/golib/redis"
)

var RedisClient *redis.Redis

// 初始化redis
func InitRedis() {
	c := conf.RConf.Redis["demo"]
	var err error
	RedisClient, err = redis.InitRedisClient(c)
	if err != nil || RedisClient == nil {
		panic("init redis failed!")
	}
}

func CloseRedis() {
	_ = RedisClient.Close()
}

// PingRedis 探测 Redis 连接可用性（超时由 ctx 控制）。
func PingRedis(ctx context.Context) error {
	if RedisClient == nil {
		return errors.New("redis client not initialized")
	}
	return RedisClient.Ping(ctx)
}

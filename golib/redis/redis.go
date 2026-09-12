// Package redis 提供基于 redigo 的 Redis 客户端最小实现，替代原内部框架 redis 包。
package redis

import (
	"context"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
)

// RedisConf Redis 连接配置。
type RedisConf struct {
	Addr            string `yaml:"addr"`
	ConnTimeOut     string `yaml:"connTimeOut"`
	IdleTimeout     string `yaml:"idleTimeout"`
	MaxActive       int    `yaml:"maxActive"`
	MaxConnLifetime string `yaml:"maxConnLifetime"`
	MaxIdle         int    `yaml:"maxIdle"`
	ReadTimeOut     string `yaml:"readTimeOut"`
	Service         string `yaml:"service"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	WriteTimeOut    string `yaml:"writeTimeOut"`
}

func parseDuration(s string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return d
	}
	return fallback
}

// Redis Redis 客户端（连接池）。
type Redis struct {
	pool *redis.Pool
}

// InitRedisClient 初始化 Redis 连接池。
func InitRedisClient(cfg RedisConf) (*Redis, error) {
	connTimeout := parseDuration(cfg.ConnTimeOut, 3*time.Second)
	readTimeout := parseDuration(cfg.ReadTimeOut, 3*time.Second)
	writeTimeout := parseDuration(cfg.WriteTimeOut, 3*time.Second)
	idleTimeout := parseDuration(cfg.IdleTimeout, 5*time.Minute)
	maxIdle := cfg.MaxIdle
	if maxIdle <= 0 {
		maxIdle = 10
	}

	pool := &redis.Pool{
		MaxIdle:     maxIdle,
		MaxActive:   cfg.MaxActive,
		IdleTimeout: idleTimeout,
		DialContext: func(ctx context.Context) (redis.Conn, error) {
			opts := []redis.DialOption{
				redis.DialConnectTimeout(connTimeout),
				redis.DialReadTimeout(readTimeout),
				redis.DialWriteTimeout(writeTimeout),
			}
			if cfg.Password != "" {
				opts = append(opts, redis.DialPassword(cfg.Password))
			}
			return redis.DialContext(ctx, "tcp", cfg.Addr, opts...)
		},
		TestOnBorrow: func(c redis.Conn, _ time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}

	conn := pool.Get()
	defer conn.Close()
	if _, err := conn.Do("PING"); err != nil {
		return nil, errors.Wrap(err, "redis ping")
	}
	return &Redis{pool: pool}, nil
}

// Get 读取字符串值；key 不存在时返回包含 "nil" 的错误。
func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	conn, err := r.pool.GetContext(ctx)
	if err != nil {
		return "", errors.Wrap(err, "redis get conn")
	}
	defer conn.Close()

	reply, err := redis.String(conn.Do("GET", key))
	if err != nil {
		return "", err
	}
	return reply, nil
}

// Set 写入字符串值并设置过期秒数。
func (r *Redis) Set(ctx context.Context, key, value string, ttlSeconds int) error {
	conn, err := r.pool.GetContext(ctx)
	if err != nil {
		return errors.Wrap(err, "redis get conn")
	}
	defer conn.Close()

	args := []interface{}{key, value}
	if ttlSeconds > 0 {
		args = append(args, "EX", ttlSeconds)
	}
	_, err = conn.Do("SET", args...)
	return err
}

// Close 关闭连接池。
func (r *Redis) Close() error {
	if r == nil || r.pool == nil {
		return nil
	}
	return r.pool.Close()
}

// Ping 探测连接可用性（带超时控制由 ctx 决定）。
func (r *Redis) Ping(ctx context.Context) error {
	if r == nil || r.pool == nil {
		return errors.New("redis client not initialized")
	}
	conn, err := r.pool.GetContext(ctx)
	if err != nil {
		return errors.Wrap(err, "redis get conn")
	}
	defer conn.Close()
	_, err = conn.Do("PING")
	return err
}

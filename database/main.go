package database

import (
	"context"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"time"
)

var c *redis.Client

func initDatabase() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	res := rdb.Ping(ctx)
	if res.Err() != nil {
		log.Fatal().Err(res.Err()).Msg("could not connect to redis")
	}

	c = rdb
}

func Database() *redis.Client {
	if c == nil {
		initDatabase()
	}
	return c
}

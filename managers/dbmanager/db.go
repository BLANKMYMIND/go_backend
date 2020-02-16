package dbmanager

import (
	"database/sql"
	"github.com/gomodule/redigo/redis"
	_ "github.com/lib/pq"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var redisHost = ":9683"
var connStr = "dbname=application user=postgres sslmode=disable"

var redisPool *redis.Pool

func init() {
	redisPool = &redis.Pool{
		MaxActive: 400,
		Wait:      true,

		MaxIdle:     30,
		IdleTimeout: 240 * time.Second,

		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", redisHost)
			if err != nil {
				return nil, err
			}
			return c, err
		},

		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	signal.Notify(c, syscall.SIGKILL)
	go func() {
		<-c
		redisPool.Close()
		os.Exit(0)
	}()
}

func DialPG() (db *sql.DB, err error) {
	db, err = sql.Open("postgres", connStr)
	return
}

func dialRedis() (db redis.Conn) {
	return redisPool.Get()
}

func SetCacheWithPX(name string, value string, px int) (res string, err error) {
	db := dialRedis()
	defer db.Close()
	res, err = redis.String(db.Do("set", name, value, "PX", px))
	return
}

func GetCache(name string) (res string, err error) {
	db := dialRedis()
	defer db.Close()
	res, err = redis.String(db.Do("get", name))
	return
}

func DelCache(name string) (res string, err error) {
	db := dialRedis()
	defer db.Close()
	res, err = redis.String(db.Do("del", name))
	return
}

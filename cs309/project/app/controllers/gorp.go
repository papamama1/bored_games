package controllers

import (
	// "code.google.com/p/go.crypto/bcrypt"
	"cs309/project/app/models"
	"database/sql"
	"github.com/coopernurse/gorp"
	"github.com/garyburd/redigo/redis"
	_ "github.com/go-sql-driver/mysql"
	r "github.com/revel/revel"
	"github.com/revel/revel/modules/db/app"
	"time"
)

var (
	DbMap     *gorp.DbMap
	RedisPool redis.Pool
)

func InitDB() {
	db.Init()

	DbMap = &gorp.DbMap{Db: db.Db, Dialect: gorp.MySQLDialect{"InnoDB", "utf8"}}

	// create mappings
	_ = DbMap.AddTable(models.User{}).SetKeys(true, "Id")
	_ = DbMap.AddTable(models.Friendship{}).SetKeys(false, "UserA", "UserB")
	_ = DbMap.AddTable(models.Announcement{}).SetKeys(true, "Id")
	t := DbMap.AddTable(models.Game{}).SetKeys(true, "Id")
	t.ColMap("Owner").Transient = true
	t = DbMap.AddTable(models.GameResult{}).SetKeys(false, "RoomId", "PlayerId")
	t.ColMap("Player").Transient = true

	DbMap.TraceOn("[gorp]", r.INFO)
	DbMap.CreateTablesIfNotExists()

	// connect Redis
	addr, found := r.Config.String("redis.address")
	if !found {
		r.ERROR.Fatal("No redis.address found.")
	}

	RedisPool = redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", addr)
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
}

type GorpController struct {
	*r.Controller
	Txn   *gorp.Transaction
	Redis redis.Conn
}

func (c *GorpController) Begin() r.Result {
	txn, err := DbMap.Begin()
	if err != nil {
		panic(err)
	}
	c.Txn = txn

	c.Redis = RedisPool.Get()
	return nil
}

func (c *GorpController) Commit() r.Result {
	if c.Txn == nil {
		return nil
	}
	if err := c.Txn.Commit(); err != nil && err != sql.ErrTxDone {
		panic(err)
	}
	c.Txn = nil

	c.Redis.Close()
	return nil
}

func (c *GorpController) Rollback() r.Result {
	if c.Txn == nil {
		return nil
	}
	if err := c.Txn.Rollback(); err != nil && err != sql.ErrTxDone {
		panic(err)
	}
	c.Txn = nil

	c.Redis.Close()
	return nil
}

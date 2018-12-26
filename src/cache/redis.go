package cache

import (
	"strconv"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
)

type Conf struct {
	Hosts string `json:"hosts"`
	Ports string `json:"ports"`
}

var defaultPools []*redis.Pool

func Init(cf *Conf) {
	hosts := strings.Split(cf.Hosts, ",")
	ports := strings.Split(cf.Ports, ",")

	if len(hosts) != len(ports) {
		panic("redis hosts != ports")
	}

	for _, port := range ports {
		if i, err := strconv.Atoi(port); err != nil {
			panic("redis port[" + strconv.Itoa(i) + "] not number: " + port)
		}
	}

	for i := 0; i != len(hosts); i++ {
		func(host, port string) {
			defaultPools = append(defaultPools, &redis.Pool{
				MaxIdle:     256,
				IdleTimeout: 240 * time.Second,
				Dial: func() (redis.Conn, error) {
					c, err := redis.DialTimeout("tcp", host+":"+port,
						50*time.Millisecond, 50*time.Millisecond, 50*time.Millisecond)
					if err != nil {
						return nil, err
					}
					return c, nil
				},
				TestOnBorrow: func(c redis.Conn, t time.Time) error {
					if time.Since(t) < 10*time.Second {
						return nil
					}
					_, err := c.Do("PING")
					return err
				},
			})
		}(hosts[i], ports[i])
	}
}

func GetConn(userHash int) redis.Conn {
	n := userHash % len(defaultPools)
	return defaultPools[n].Get()
}

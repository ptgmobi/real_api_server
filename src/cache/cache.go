package cache

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"

	"set"
)

func Set(userHash int, key, val string, expire int) error {
	if len(key) == 0 || len(val) == 0 || expire < 0 {
		return errors.New("[Set] empty key or val or invalid expire")
	}

	conn := GetConn(userHash)
	defer conn.Close()

	if err := conn.Send("SET", key, val); err != nil {
		return err
	}
	if err := conn.Send("EXPIRE", key, expire); err != nil {
		return err
	}
	return conn.Flush()
}

func Get(userHash int, key string) (string, error) {
	if len(key) == 0 {
		return "", errors.New("[GET] empty key")
	}

	conn := GetConn(userHash)
	defer conn.Close()

	return redis.String(conn.Do("GET", key))
}

func Del(userHash int, key string) (string, error) {
	if len(key) == 0 {
		return "", errors.New("[DEL] empty key")
	}

	conn := GetConn(userHash)
	defer conn.Close()

	return redis.String(conn.Do("DEL", key))
}

func IncrFreq(userHash int, key string, fields []string, expire int64) error {
	if len(key) == 0 || len(fields) == 0 || expire < 0 {
		return errors.New("[IncrPreClickFreq] empty key or fields or invalid expire")
	}

	conn := GetConn(userHash)
	defer conn.Close()

	for _, field := range fields {
		if err := conn.Send("HINCRBY", key, field, 1); err != nil {
			return fmt.Errorf("HINCRBY (%s %s) error: %v ", key, field, err)
		}
	}

	if err := conn.Send("EXPIRE", key, int(expire)); err != nil {
		return fmt.Errorf("EXPIRE (%s %s) error: %v", key, strconv.FormatInt(expire, 10), err)
	}

	if err := conn.Flush(); err != nil {
		return fmt.Errorf("FLUSH (%s %s) error: %v", key, strings.Join(fields, " "), err)
	}

	return nil
}

func HGetAllFreq(userHash int, key string) (map[string]int, error) {
	if len(key) == 0 {
		return nil, errors.New("empty key")
	}

	conn := GetConn(userHash)
	defer conn.Close()

	return redis.IntMap(conn.Do("HGETALL", key))
}

func HGetFreq(userHash int, key, field string) (int, error) {
	if len(key) == 0 {
		return 0, errors.New("empty key")
	}

	conn := GetConn(userHash)
	defer conn.Close()

	return redis.Int(conn.Do("HGET", key, field))
}

func GetAndSetVideoCtrlInfos(userHash int, serverCidKey, convKey, vCompleteKey, vRequestKey,
	vFreqField string) (map[string]int, *set.Set, int, int, error) {

	if len(serverCidKey) == 0 && len(convKey) == 0 && len(vCompleteKey) == 0 && len(vRequestKey) == 0 {
		return nil, nil, 0, 0, errors.New("empty key")
	}

	conn := GetConn(userHash)
	defer conn.Close()

	if err := conn.Send("HGETALL", serverCidKey); err != nil {
		return nil, nil, 0, 0, fmt.Errorf("HGETALL (%s) error: %v ", serverCidKey, err)
	}

	if len(convKey) > 0 {
		if err := conn.Send("ZRANGE", convKey, 0, -1); err != nil {
			return nil, nil, 0, 0, fmt.Errorf("ZRANGE (%s) error: %v ", convKey, err)
		}
	}

	if len(vFreqField) > 0 {
		if err := conn.Send("HGET", vCompleteKey, vFreqField); err != nil {
			return nil, nil, 0, 0, fmt.Errorf("HGET (%s %s) error: %v ", vCompleteKey, vFreqField, err)
		}

		if err := conn.Send("HGET", vRequestKey, vFreqField); err != nil {
			return nil, nil, 0, 0, fmt.Errorf("HGET (%s %s) error: %v ", vRequestKey, vFreqField, err)
		}

		if err := conn.Send("HINCRBY", vRequestKey, vFreqField, 1); err != nil { // 增加该用户激励视频请求次数
			return nil, nil, 0, 0, fmt.Errorf("HINCRBY (%s %s) error: %v ", vRequestKey, vFreqField, err)
		}

		expire := 86400 - time.Now().Unix()%86400
		if err := conn.Send("EXPIRE", vRequestKey, int(expire)); err != nil {
			return nil, nil, 0, 0, fmt.Errorf("EXPIRE (%s %s) error: %v", vRequestKey, strconv.FormatInt(expire, 10), err)
		}
	}

	if err := conn.Flush(); err != nil {
		return nil, nil, 0, 0, fmt.Errorf("FLUSH (%s %s %s %s) error: %v", serverCidKey, convKey, vCompleteKey, vRequestKey, err)
	}

	serverCidMap, _ := redis.IntMap(conn.Receive())

	convSet := set.NewSet()
	if len(convKey) > 0 {
		convPkgs, _ := redis.Strings(conn.Receive())
		for _, v := range convPkgs {
			convSet.Add(v)
		}
	}

	var vCompletes, vRequests int
	if len(vFreqField) > 0 {
		vCompletes, _ = redis.Int(conn.Receive())
		vRequests, _ = redis.Int(conn.Receive())
	}

	return serverCidMap, convSet, vCompletes, vRequests, nil
}

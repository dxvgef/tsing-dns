package redis

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"local/global"

	"github.com/go-redis/redis/v8"
	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

type Redis struct {
	config *Config
	cli    *redis.Client
}

type Config struct {
	Addr     string `json:"addr"`
	Database int    `json:"database,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Prefix   string `json:"prefix,omitempty"`
	Timeout  uint16 `json:"timeout,omitempty"`
}

func New(config *Config) (*Redis, error) {
	var inst Redis
	inst.config = config
	if inst.config.Timeout == 0 {
		inst.config.Timeout = 5
	}
	inst.cli = redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Username: config.Username,
		Password: config.Password,
		DB:       config.Database,
	})
	return &inst, nil
}

func NewWithJSON(jsonStr string) (*Redis, error) {
	var (
		err    error
		config Config
	)
	err = json.Unmarshal(global.StrToBytes(jsonStr), &config)
	if err != nil {
		return nil, err
	}
	return New(&config)
}

func (inst *Redis) Set(rr dns.RR) (err error) {
	var (
		key     strings.Builder
		keySign string
		expires time.Duration
	)

	if !strings.HasSuffix(rr.Header().Name, ".") {
		rr.Header().Name += "."
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(inst.config.Timeout*2)*time.Second)
	defer cancel()

	keySign, err = global.KeySign(rr)
	if err != nil {
		return
	}

	key.WriteString(inst.config.Prefix)
	key.WriteString(rr.Header().Name)
	key.WriteString(":")
	key.WriteString(dns.ClassToString[rr.Header().Class])
	key.WriteString("-")
	key.WriteString(dns.TypeToString[rr.Header().Rrtype])
	key.WriteString(":")
	key.WriteString(keySign)

	if global.Config.Storage.UseExpire {
		expires = time.Duration(rr.Header().Ttl) * time.Second
	}

	err = inst.cli.HSet(ctx, key.String(), "r_name", rr.Header().Name, "r_class", dns.ClassToString[rr.Header().Class], "r_type", dns.TypeToString[rr.Header().Rrtype], "r_ttl", rr.Header().Ttl, "r_data", strings.TrimPrefix(rr.String(), rr.Header().String())).Err()
	if err != nil {
		log.Err(err).Caller().Msg("Redis写入记录")
		return
	}

	if expires > 0 {
		err = inst.cli.Expire(ctx, key.String(), expires).Err()
		if err != nil {
			log.Err(err).Caller().Msg("Redis设置记录TTL")
			return
		}
	}
	return
}

func (inst *Redis) Get(question dns.Question) ([]dns.RR, error) {
	var (
		err    error
		value  map[string]string
		result []dns.RR
		rr     dns.RR
		key    strings.Builder
		keys   []string
		rStr   strings.Builder
	)

	if !strings.HasSuffix(question.Name, ".") {
		question.Name += "."
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(inst.config.Timeout)*time.Second)
	defer cancel()

	key.WriteString(inst.config.Prefix)
	key.WriteString(question.Name)
	key.WriteString(":")
	key.WriteString(dns.ClassToString[question.Qclass])
	key.WriteString("-")
	key.WriteString(dns.TypeToString[question.Qtype])
	key.WriteString(":*")

	keys, err = inst.cli.Keys(ctx, key.String()).Result()
	if err != nil {
		return nil, err
	}

	for k := range keys {
		rStr.Reset()
		value, err = inst.cli.HGetAll(ctx, keys[k]).Result()
		if err != nil {
			return nil, err
		}
		rStr.WriteString(value["r_name"])
		rStr.WriteString(" ")
		rStr.WriteString(value["r_ttl"])
		rStr.WriteString(" ")
		rStr.WriteString(value["r_class"])
		rStr.WriteString(" ")
		rStr.WriteString(value["r_type"])
		rStr.WriteString(" ")
		rStr.WriteString(value["r_data"])
		rr, err = dns.NewRR(rStr.String())
		if err != nil {
			return nil, err
		}
		result = append(result, rr)
	}

	return result, nil
}

func (inst *Redis) Del(rr dns.RR) (err error) {
	if !strings.HasSuffix(rr.Header().Name, ".") {
		rr.Header().Name += "."
	}

	var key strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(inst.config.Timeout)*time.Second)
	defer cancel()
	name := strings.TrimSuffix(rr.Header().Name, ".")
	keySign, err := global.KeySign(rr)
	key.WriteString(inst.config.Prefix)
	key.WriteString(name)
	key.WriteString(":")
	key.WriteString(dns.ClassToString[rr.Header().Class])
	key.WriteString("-")
	key.WriteString(dns.TypeToString[rr.Header().Rrtype])
	key.WriteString(":")
	key.WriteString(keySign)
	return inst.cli.Del(ctx, key.String()).Err()
}

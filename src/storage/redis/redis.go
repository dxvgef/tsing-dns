package redis

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"local/global"

	"github.com/go-redis/redis/v8"
	"github.com/miekg/dns"
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
	UseTTL   bool   `json:"useTTL,omitempty"`
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
		name    string
		key     string
		keySign string
		expires time.Duration
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(inst.config.Timeout*2)*time.Second)
	defer cancel()

	name = strings.TrimSuffix(rr.Header().Name, ".")
	keySign, err = global.KeySign(rr)
	if err != nil {
		return
	}
	if inst.config.UseTTL {
		expires = time.Duration(rr.Header().Ttl) * time.Second
	}
	key = inst.config.Prefix + name + ":" + dns.ClassToString[rr.Header().Class] + "-" + dns.TypeToString[rr.Header().Rrtype] + ":" + keySign
	err = inst.cli.Set(ctx, key, rr.String(), expires).Err()
	return
}

func (inst *Redis) Get(question dns.Question) ([]dns.RR, error) {
	var (
		err    error
		data   string
		result []dns.RR
		rr     dns.RR
		keys   []string
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(inst.config.Timeout)*time.Second)
	defer cancel()
	name := strings.TrimSuffix(question.Name, ".")
	key := inst.config.Prefix + name + ":" + dns.ClassToString[question.Qclass] + "-" + dns.TypeToString[question.Qtype] + ":*"

	keys, err = inst.cli.Keys(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	for k := range keys {
		data, err = inst.cli.Get(ctx, keys[k]).Result()
		if err != nil {
			return nil, err
		}
		rr, err = dns.NewRR(data)
		if err != nil {
			return nil, err
		}
		result = append(result, rr)
	}

	return result, nil
}

func (inst *Redis) Del(rr dns.RR) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(inst.config.Timeout)*time.Second)
	defer cancel()
	name := strings.TrimSuffix(rr.Header().Name, ".")
	keySign, err := global.KeySign(rr)
	key := inst.config.Prefix + name + ":" + dns.ClassToString[rr.Header().Class] + "-" + dns.TypeToString[rr.Header().Rrtype] + ":" + keySign
	return inst.cli.Del(ctx, key).Err()
}

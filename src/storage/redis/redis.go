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
	UseTTL   bool   `json:"useTTL,omitempty"`
}

func New(config *Config) (*Redis, error) {
	var inst Redis
	inst.config = config
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

func (inst *Redis) Set(rr dns.RR, ttl uint32) (err error) {
	var (
		name    string
		key     string
		keySign string
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	name = strings.TrimSuffix(rr.Header().Name, ".")
	keySign, err = global.KeySign(rr)
	if err != nil {
		return
	}
	key = inst.config.Prefix + name + ":" + dns.TypeToString[rr.Header().Rrtype] + ":" + keySign
	err = inst.cli.Set(ctx, key, rr.String(), time.Duration(ttl)*time.Second).Err()
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	name := strings.TrimSuffix(question.Name, ".")
	key := inst.config.Prefix + name + ":" + dns.TypeToString[question.Qtype] + ":*"

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	name := strings.TrimSuffix(rr.Header().Name, ".")
	key := inst.config.Prefix + name + ":" + dns.TypeToString[rr.Header().Rrtype]

	return inst.cli.Del(ctx, key).Err()
}
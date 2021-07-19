package voltdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"local/global"

	"github.com/miekg/dns"
	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
)

type VoltDB struct {
	config *Config
	cli    *sql.DB
}

type Config struct {
	Addr     string `json:"addr"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseTTL   bool   `json:"useTTL,omitempty"`
}

func New(config *Config) (*VoltDB, error) {
	var (
		inst VoltDB
		err  error
	)
	inst.config = config
	if config.Username != "" {
		inst.cli, err = sql.Open("voltdb", "voltdb://"+config.Username+":"+config.Password+"@"+config.Addr)
	} else {
		inst.cli, err = sql.Open("voltdb", "voltdb://"+config.Addr)
	}
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = inst.cli.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return &inst, nil
}

func NewWithJSON(jsonStr string) (*VoltDB, error) {
	var (
		config Config
		err    error
	)
	err = json.Unmarshal(global.StrToBytes(jsonStr), &config)
	if err != nil {
		return nil, err
	}

	return New(&config)
}

func (inst *VoltDB) Set(rr dns.RR, ttl uint32) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id := xid.New().String()
	name := strings.TrimSuffix(rr.Header().Name, ".")
	_, err = inst.cli.ExecContext(ctx, "@AdHoc", "INSERT INTO domain (id, rr_name, rr_type, rr_ttl, rr_data) VALUES (?, ?, ?, ?, ?)", id, name, rr.Header().Rrtype, rr.Header().Ttl, rr.String())
	return
}

func (inst *VoltDB) Get(question dns.Question) ([]dns.RR, error) {
	var (
		err    error
		data   string
		result []dns.RR
		rows   *sql.Rows
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	name := strings.TrimSuffix(question.Name, ".")

	rows, err = inst.cli.QueryContext(ctx, "@AdHoc", "select rr_data from domain WHERE rr_name=? AND rr_type=?", name, question.Qtype)
	if err != nil {
		return nil, err
	}
	if rows.Err() != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Warn().Err(err).Caller().Send()
		}
	}()
	for rows.Next() {
		var rr dns.RR
		err = rows.Scan(&data)
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

func (inst *VoltDB) Del(rr dns.RR) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	name := strings.TrimSuffix(rr.Header().Name, ".")
	_, err = inst.cli.ExecContext(ctx, "@AdHoc", "DELETE FROM domain WHERE rr_name=? AND rr_type=?", name, dns.TypeToString[rr.Header().Rrtype])
	return
}

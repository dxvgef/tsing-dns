package voltdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"local/global"

	_ "github.com/VoltDB/voltdb-client-go/voltdbclient"
	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

type VoltDB struct {
	config *Config
	cli    *sql.DB
}

type Config struct {
	Addr            string `json:"addr"`
	Table           string `json:"table"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	Timeout         uint16 `json:"timeout,omitempty"`
	CleanupInterval int    `json:"cleanupInterval,omitempty"`
}

func New(config *Config) (*VoltDB, error) {
	var (
		inst VoltDB
		err  error
	)
	inst.config = config
	if inst.config.Timeout == 0 {
		inst.config.Timeout = 5
	}
	if config.Username != "" {
		inst.cli, err = sql.Open("voltdb", "voltdb://"+config.Username+":"+config.Password+"@"+config.Addr)
	} else {
		inst.cli, err = sql.Open("voltdb", "voltdb://"+config.Addr)
	}
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

func (inst *VoltDB) Set(rr dns.RR) (err error) {
	var (
		expired int64
		exist   bool
		rrData  string
	)

	if !strings.HasSuffix(rr.Header().Name, ".") {
		rr.Header().Name += "."
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(inst.config.Timeout)*time.Second)
	defer cancel()

	if global.Config.Storage.UseExpire {
		expired = time.Now().Add(time.Duration(rr.Header().Ttl) * time.Second).Unix()
	}

	rrData = strings.TrimPrefix(rr.String(), rr.Header().String())

	err = inst.cli.QueryRowContext(ctx, "@AdHoc", "select 1 from "+inst.config.Table+" WHERE r_name=? AND r_class=? AND r_type=? AND r_data=?", rr.Header().Name, rr.Header().Class, rr.Header().Rrtype, rrData).Scan(&exist)
	if err != nil {
		if err.Error() != sql.ErrNoRows.Error() {
			log.Err(err).Caller().Msg("VoltDB存储器查询记录是否存在")
			return
		}
	}

	if exist {
		_, err = inst.cli.ExecContext(ctx, "@AdHoc", "UPDATE "+inst.config.Table+" SET expired_at=?, r_ttl=? WHERE r_name=? AND r_class=? AND r_type=?", expired, rr.Header().Ttl, rr.Header().Name, rr.Header().Class, rr.Header().Rrtype)
	} else {
		_, err = inst.cli.ExecContext(ctx, "@AdHoc", "INSERT INTO "+inst.config.Table+" (r_data, r_name, r_class, r_type, r_ttl, expired_at) VALUES (?, ?, ?, ?, ?, ?)", rrData, rr.Header().Name, rr.Header().Class, rr.Header().Rrtype, rr.Header().Ttl, expired)
	}

	if err != nil {
		log.Err(err).Caller().Msg("VoltDB存储器写入记录")
		return
	}

	if err = inst.cleanupExpired(); err != nil {
		log.Err(err).Caller().Msg("VoltDB存储器自动清理已过期记录")
		return
	}

	return
}

func (inst *VoltDB) Get(question dns.Question) ([]dns.RR, error) {
	var (
		err    error
		rName  string
		rClass uint16
		rType  uint16
		rTTL   int
		rData  string
		rStr   strings.Builder
		result []dns.RR
		rows   *sql.Rows
	)

	if !strings.HasSuffix(question.Name, ".") {
		question.Name += "."
	}

	if err = inst.cleanupExpired(); err != nil {
		log.Err(err).Caller().Msg("VoltDB存储器自动清理已过期记录")
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(inst.config.Timeout)*time.Second)
	defer cancel()

	if global.Config.Storage.UseExpire {
		rows, err = inst.cli.QueryContext(ctx, "@AdHoc", "select r_name, r_class, r_type, r_ttl, r_data from "+inst.config.Table+" WHERE r_name=? AND r_class=? AND r_type=? AND expired_at=0 OR expired_at>?", question.Name, question.Qclass, question.Qtype, time.Now().Unix())
	} else {
		rows, err = inst.cli.QueryContext(ctx, "@AdHoc", "select r_name, r_class, r_type, r_ttl, r_data from "+inst.config.Table+" WHERE r_name=? AND r_class=? AND r_type=?", question.Name, question.Qclass, question.Qtype)
	}
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
		var (
			rr dns.RR
		)
		err = rows.Scan(&rName, &rClass, &rType, &rTTL, &rData)
		if err != nil {
			return nil, err
		}
		rStr.WriteString(rName)
		rStr.WriteString(" ")
		rStr.WriteString(strconv.Itoa(rTTL))
		rStr.WriteString(" ")
		rStr.WriteString(dns.ClassToString[rClass])
		rStr.WriteString(" ")
		rStr.WriteString(dns.TypeToString[rType])
		rStr.WriteString(" ")
		rStr.WriteString(rData)
		rr, err = dns.NewRR(rStr.String())
		if err != nil {
			return nil, err
		}
		result = append(result, rr)
		rStr.Reset()
	}

	return result, nil
}

func (inst *VoltDB) Del(rr dns.RR) (err error) {
	if !strings.HasSuffix(rr.Header().Name, ".") {
		rr.Header().Name += "."
	}

	if err = inst.cleanupExpired(); err != nil {
		log.Err(err).Caller().Msg("VoltDB存储器自动清理已过期记录")
		return
	}

	rrData := strings.TrimPrefix(rr.String(), rr.Header().String())

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(inst.config.Timeout)*time.Second)
	defer cancel()
	_, err = inst.cli.ExecContext(ctx, "@AdHoc", "DELETE FROM "+inst.config.Table+" WHERE r_data=? AND r_class AND r_type=?", rrData, rr.Header().Class, dns.TypeToString[rr.Header().Rrtype])
	return
}

// 清除已过期的记录
func (inst *VoltDB) cleanupExpired() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(inst.config.Timeout)*time.Second)
	defer cancel()
	_, err = inst.cli.ExecContext(ctx, "@AdHoc", "DELETE FROM "+inst.config.Table+" WHERE expired_at<>0 AND expired_at<?", time.Now().Unix())
	return
}

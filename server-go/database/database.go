package database

import (
	"database/sql/driver"
	"fmt"
	"log"

	ora "github.com/sijms/go-ora/v2"
)

const (
	sql_query_sessions = `select t.sid, t.serial# as serial, t.osuser, t.terminal, t.program, t.service_name, t.schemaname FROM v$session t WHERE t.service_name NOT LIKE 'SYS%%' AND t.PROGRAM != '%s' AND t.TERMINAL != 'unknown' ORDER BY t.osuser, t.SID`
	sql_get_tracefile  = `SELECT p.tracefile FROM v$session s JOIN v$process p ON s.paddr = p.addr WHERE s.sid = :SID`
	sql_enable_trace   = `BEGIN SYS.DBMS_MONITOR.SESSION_TRACE_ENABLE(:1, :2, TRUE, TRUE); END;`
	sql_disable_trace  = `BEGIN SYS.DBMS_MONITOR.SESSION_TRACE_DISABLE(:1, :2); END;`
	sql_get_encoding   = `select VALUE from nls_database_parameters@%s where parameter='NLS_CHARACTERSET'`
)

type Session struct {
	Sid      int    `json:"sid"`
	Serial   int    `json:"serial"`
	Osuser   string `json:"osuser"`
	Terminal string `json:"terminal"`
	Program  string `json:"program"`
	Service  string `json:"service"`
	Schema   string `json:"schema"`
}

type Database interface {
	Close()
	Open() error
	Tracing(sid int, serial int) bool
	UnTracing(sid int, serial int) bool
	GetTraceFile(sid int) string
	QuerySessions(executable string) []Session
	GetEncoding(service string) string
}

type OracleDatabase struct {
	Connection *ora.Connection
}

func NewOracleDatabase(connStr string) *OracleDatabase {
	if conn, err := ora.NewConnection(connStr); err == nil {
		return &OracleDatabase{Connection: conn}
	} else {
		log.Println("newOracleDatabase:", err)
		return nil
	}
}

func (db *OracleDatabase) Close() {
	db.Connection.Close()
}

func (db *OracleDatabase) Open() error {
	return db.Connection.Open()
}

func (db *OracleDatabase) Tracing(sid int, serial int) bool {
	if sid == 0 || serial == 0 {
		return false
	}

	_, err := db.Connection.Exec(sql_enable_trace, sid, serial)
	if err != nil {
		log.Println("tracing:", err)
		return false
	}
	return true
}

func (db *OracleDatabase) UnTracing(sid int, serial int) bool {
	if sid == 0 || serial == 0 {
		return false
	}

	_, err := db.Connection.Exec(sql_disable_trace, sid, serial)
	if err != nil {
		log.Println("untracing:", err)
		return false
	}
	return true
}

func (db *OracleDatabase) GetTraceFile(sid int) string {
	stmt := ora.NewStmt(sql_get_tracefile, db.Connection)
	stmt.AddParam("SID", sid, 32, ora.Input)
	defer stmt.Close()

	if rows, err := stmt.Query(nil); err != nil {
		return ""
	} else {
		vals := make([]driver.Value, len(rows.Columns()))
		if err := rows.Next(vals); err != nil {
			return ""
		} else {
			return vals[0].(string)
		}
	}
}

func (db *OracleDatabase) GetEncoding(service string) string {
	stmt := ora.NewStmt(fmt.Sprintf(sql_get_encoding, service), db.Connection)
	defer stmt.Close()

	if rows, err := stmt.Query(nil); err != nil {
		return "AL32UTF8"
	} else {
		vals := make([]driver.Value, len(rows.Columns()))
		if err := rows.Next(vals); err != nil {
			return "AL32UTF8"
		} else {
			return vals[0].(string)
		}
	}
}

func (db *OracleDatabase) QuerySessions(executable string) []Session {
	stmt := ora.NewStmt(fmt.Sprintf(sql_query_sessions, executable), db.Connection)
	defer stmt.Close()

	var sessions []Session
	if rows, err := stmt.Query(nil); err != nil {
		log.Println("querySessions:", err)
		return nil
	} else {
		values := make([]driver.Value, len(rows.Columns()))
		for {
			if err := rows.Next(values); err != nil {
				return sessions
			}
			var session = Session{
				Sid:      int(values[0].(int64)),
				Serial:   int(values[1].(int64)),
				Osuser:   values[2].(string),
				Terminal: values[3].(string),
				Program:  values[4].(string),
				Service:  values[5].(string),
				Schema:   values[6].(string),
			}
			sessions = append(sessions, session)
		}
	}
}

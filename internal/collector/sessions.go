package collector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

const sqlSessionsQuery = `
SELECT * FROM (
    SELECT
        s.inst_id,
        s.sid,
        s.serial#,
        NVL(s.username, 'BG_PROC'),
        s.status,
        NVL(s.schemaname, 'SYS'),
        NVL(SUBSTR(s.machine, 1, 24), '-'),
        NVL(SUBSTR(s.program, 1, 24), '-'),
        NVL(s.sql_id, '-'),
        NVL(s.wait_class, 'Idle'),
        NVL(SUBSTR(s.event, 1, 32), 'ON CPU'),
        NVL(s.last_call_et, 0),
        CASE
            WHEN s.blocking_session IS NOT NULL THEN 'INST:' || s.blocking_instance || ' SID:' || s.blocking_session
            WHEN EXISTS (SELECT 1 FROM gv$session b WHERE b.blocking_instance = s.inst_id AND b.blocking_session = s.sid) THEN 'BLOCKER'
            ELSE '-'
        END AS blocker_info,
        TO_CHAR(s.logon_time, 'HH24:MI:SS'),
        ROUND(NVL(p.pga_alloc_mem, 0)/1024/1024, 1) AS pga_mb,
        NVL(REPLACE(REPLACE(SUBSTR(q.sql_text, 1, 160), CHR(10), ' '), '|', ' '), '-') AS sql_snippet
    FROM gv$session s
    LEFT JOIN gv$process p ON p.inst_id = s.inst_id AND p.addr = s.paddr
    LEFT JOIN gv$sql q ON q.inst_id = s.inst_id AND q.sql_id = s.sql_id AND q.child_number = NVL(s.sql_child_number, 0)
    WHERE s.type = 'USER' AND s.username IS NOT NULL
    ORDER BY
        CASE WHEN s.blocking_session IS NOT NULL THEN 0 WHEN s.status = 'ACTIVE' THEN 1 ELSE 2 END,
        s.last_call_et DESC
) WHERE ROWNUM <= 100;
`

// ParseSessionRows converts pipe-delimited SQL rows into UserSession structs.
func ParseSessionRows(rows [][]string) []model.UserSession {
	var sessions []model.UserSession
	for _, r := range rows {
		if len(r) < 16 {
			continue
		}
		instID, _ := strconv.Atoi(r[0])
		sid, _ := strconv.Atoi(r[1])
		serial, _ := strconv.Atoi(r[2])
		elapsed, _ := strconv.Atoi(r[11])
		pgaMB, _ := strconv.ParseFloat(r[14], 64)
		sessions = append(sessions, model.UserSession{
			InstID:          instID,
			SID:             sid,
			Serial:          serial,
			Username:        r[3],
			Status:          r[4],
			Schema:          r[5],
			Machine:         r[6],
			Program:         r[7],
			SQLID:           r[8],
			WaitClass:       r[9],
			WaitEvent:       r[10],
			ElapsedSec:      elapsed,
			BlockingSession: r[12],
			LogonTime:       r[13],
			PGAMB:           pgaMB,
			SQLText:         fmt.Sprintf("%s", r[15]),
		})
	}
	return sessions
}

// CollectSessions queries gv$session for active/inactive user sessions, locks, and SQL text.
func CollectSessions(ctx context.Context, runner *executor.Runner, env *discovery.DiscoveredEnvironment, sid string) []model.UserSession {
	if env.OracleHome == "" || sid == "" {
		return nil
	}
	rows, _, err := runner.RunSQLPlus(ctx, env.OracleHome, sid, env.OracleOwner, sqlSessionsQuery, nil)
	if err != nil {
		return nil
	}
	return ParseSessionRows(rows)
}

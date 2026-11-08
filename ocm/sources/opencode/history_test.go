package opencode

import (
	"context"
	"errors"
	"fmt"
	"github.com/kimoofey/tui/ocm/session"
	"strings"
	"testing"
)

func usageSum(o session.Observation) float64 {
	var total float64
	for _, u := range o.Usage {
		if u.Cost != nil {
			total += *u.Cost
		}
	}
	return total
}
func TestHistoryV1RecordedForksAndValidation(t *testing.T) {
	s := createTestStore(t, testSessions)
	w := testWriter(t, s.Path())
	execTestSQL(t, w, `CREATE TABLE message(id TEXT,session_id TEXT,time_created INTEGER,data TEXT);
 INSERT INTO message VALUES ('original','sess-1',0,'{"role":"assistant","cost":3,"providerID":"p","modelID":"m","tokens":{"input":0}}'),('copy','sess-2',86400000,'{"role":"assistant","cost":3}'),('new','sess-2',NULL,'{"role":"assistant","cost":1}'),('orphan','missing',0,'{"role":"assistant","cost":90}');
 UPDATE session SET parent_id='sess-1' WHERE id='sess-2';`)
	ids, err := s.HistorySessions(context.Background())
	if err != nil || len(ids) != 3 {
		t.Fatal(ids, err)
	}
	var total float64
	for _, id := range ids {
		o, err := s.CaptureHistory(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if !o.ApproximateV1 || !o.Present {
			t.Fatal(o)
		}
		total += usageSum(o)
	}
	if total != 7 {
		t.Fatalf("recorded=%v; actual-call reference=4", total)
	}
	o, err := s.CaptureHistory(context.Background(), "sess-1")
	if err != nil || *o.Usage[0].Input != 0 {
		t.Fatal(o, err)
	}
	o, err = s.CaptureHistory(context.Background(), "sess-3")
	if err != nil || !o.Present || len(o.Usage) != 0 {
		t.Fatal(o, err)
	}
	o, err = s.CaptureHistory(context.Background(), "missing")
	if err != nil || o.Present {
		t.Fatal(o, err)
	}
	// Orphan messages never create an observation of an absent session.
	if len(o.Usage) != 0 {
		t.Fatal("orphan usage included")
	}
	for _, data := range []string{`{`, `{"role":3}`, `{"role":"mystery"}`} {
		execTestSQL(t, w, `UPDATE message SET data=? WHERE id='original'`, data)
		if _, err = s.CaptureHistory(context.Background(), "sess-1"); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
	execTestSQL(t, w, `UPDATE message SET data='{"role":"assistant","cost":"bad","tokens":{"input":-1,"output":9223372036854775808}}' WHERE id='original'`)
	o, err = s.CaptureHistory(context.Background(), "sess-1")
	if err != nil || o.Usage[0].Cost != nil || o.Usage[0].Input != nil || o.Usage[0].Output != nil {
		t.Fatal(o, err)
	}
	execTestSQL(t, w, `INSERT INTO message SELECT * FROM message WHERE id='original'`)
	if _, err = s.CaptureHistory(context.Background(), "sess-1"); err == nil {
		t.Fatal("duplicate accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.CaptureHistory(ctx, "sess-1"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	execTestSQL(t, w, `DROP TABLE message`)
	if _, err = s.CaptureHistory(context.Background(), "sess-1"); err == nil {
		t.Fatal("missing usage table accepted")
	}
}
func TestHistoryV2Oracle(t *testing.T) {
	s, w := createV2Store(t)
	execTestSQL(t, w, `INSERT INTO session_v2 VALUES ('root','Root','/p',1000,2000,0,NULL,NULL),('fork','Fork','/p',2000,3000,0,NULL,'root');
 INSERT INTO session_message VALUES ('old','root','assistant',1,1000,1000,'{"cost":2.5,"model":{"providerID":"p","id":"m"}}'),('copied','fork','assistant',1,1000,1000,'{"cost":2.5}'),('new','fork','assistant',2,2000,2000,'{"cost":3}'),('user','root','user',2,2500,2500,'{}');
 INSERT INTO event VALUES ('compact','root',3,'session.usage.recorded.1','{"source":"compaction","cost":1}',1500),('other','root',4,'session.usage.recorded.1','{"source":"assistant","cost":50}',1500);`)
	var all []session.Usage
	for _, id := range []string{"root", "fork"} {
		o, err := s.CaptureHistory(context.Background(), id)
		if err != nil || o.ApproximateV1 {
			t.Fatal(o, err)
		}
		all = append(all, o.Usage...)
	}
	for _, tc := range []struct {
		start, end int64
		want       float64
	}{{0, 4000, 6.5}, {2000, 3000, 3}, {3000, 4000, 0}} {
		var total float64
		for _, u := range all {
			if u.Timestamp != nil && *u.Timestamp >= tc.start && *u.Timestamp < tc.end && u.Cost != nil {
				total += *u.Cost
			}
		}
		if total != tc.want {
			t.Fatal(tc, total)
		}
	}
	execTestSQL(t, w, `UPDATE event SET data='{"source":null}' WHERE id='compact'`)
	if _, err := s.CaptureHistory(context.Background(), "root"); err == nil {
		t.Fatal("uncertain compaction accepted")
	}
	execTestSQL(t, w, `DELETE FROM session_v2`)
	ids, err := s.HistorySessions(context.Background())
	if err != nil || len(ids) != 0 {
		t.Fatal(ids, err)
	}
}

func TestHistoryV2ControlMessages(t *testing.T) {
	s, w := createV2Store(t)
	execTestSQL(t, w, `INSERT INTO session_v2 VALUES('root','Root','/p',1000,2000,0,NULL,NULL)`)
	// From the pinned upstream SessionMessage.Info union; compaction accounting is
	// supplied by usage events even when a projection also carries a cost.
	for i, kind := range []string{"agent-switched", "model-switched", "location-switched", "synthetic", "skill", "shell", "compaction", "idle"} {
		execTestSQL(t, w, `INSERT INTO session_message VALUES(?, 'root', ?, ?,1000,1000,'{"cost":99}')`, fmt.Sprint(i), kind, i)
	}
	execTestSQL(t, w, `INSERT INTO event VALUES('compaction-cost','root',1,'session.usage.recorded.1','{"source":"compaction","cost":2}',1000)`)
	o, err := s.CaptureHistory(context.Background(), "root")
	if err != nil || usageSum(o) != 2 || len(o.Usage) != 1 {
		t.Fatal(o, err)
	}
	execTestSQL(t, w, `INSERT INTO session_message VALUES('unknown','root','future-type',100,1000,1000,'{}')`)
	_, err = s.CaptureHistory(context.Background(), "root")
	if err == nil || !strings.Contains(err.Error(), `session "root" message "unknown"`) {
		t.Fatal(err)
	}
}
func TestHistoryDeletionScope(t *testing.T) {
	s, w := createV2Store(t)
	execTestSQL(t, w, `INSERT INTO session_v2 VALUES ('root','Root','/p',1000,2000,0,NULL,NULL),('child','Child','/p',1000,2000,0,'root',NULL),('grand','Grand','/p',1000,2000,0,'child',NULL),('fork','Fork','/p',1000,2000,0,NULL,'root')`)
	ctx := context.Background()
	if !s.HistoryDeletionAvailability(ctx).Available {
		t.Fatal("V2 unavailable")
	}
	refs, err := s.HistoryDeletionScope(ctx, session.Ref{Source: s.Info().ID, ID: "root"})
	if err != nil || len(refs) != 3 {
		t.Fatal(refs, err)
	}
	if _, err = s.HistoryDeletionScope(ctx, session.Ref{Source: "foreign", ID: "root"}); !errors.Is(err, session.ErrForeignSource) {
		t.Fatal(err)
	}
	if _, err = s.HistoryDeletionScope(ctx, session.Ref{Source: s.Info().ID, ID: "absent"}); err == nil {
		t.Fatal("absent target accepted")
	}
	execTestSQL(t, w, `UPDATE session_v2 SET parent_id='grand' WHERE id='root'`)
	if _, err = s.HistoryDeletionScope(ctx, session.Ref{Source: s.Info().ID, ID: "root"}); !errors.Is(err, session.ErrUnsupported) {
		t.Fatal(err)
	}
	v1 := createTestStore(t, testSessions)
	if v1.HistoryDeletionAvailability(ctx).Available {
		t.Fatal("guessed project-dependent V1 scope")
	}
}

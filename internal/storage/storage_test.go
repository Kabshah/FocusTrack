package storage

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStorage(t *testing.T) {
	dbPath := "test_final.db"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)

	db, err := Open(dbPath)
	assert.NoError(t, err)
	defer db.Close()



	err = Migrate(db)
	assert.NoError(t, err)

	now := time.Now()
	year, month, day := now.Date()
	loc := now.Location()
	dayStart := time.Date(year, month, day, 0, 0, 0, 0, loc)
	
	// Use a timestamp that is definitively within "today"
	testTime := dayStart.Add(2 * time.Hour).Unix()
	
	t.Logf("Day Start: %d, Test Time: %d", dayStart.Unix(), testTime)
	
	id, err := InsertSession(db, "TestApp", "test.exe", "Test Window", testTime)
	assert.NoError(t, err)
	assert.Greater(t, id, int64(0))

	err = CloseSession(db, id, testTime+60) // 1 minute session
	assert.NoError(t, err)

	stats, err := GetDailyStats(db, now)
	assert.NoError(t, err)
	
	if len(stats) == 0 {
		var count int
		_ = db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
		var startAt, duration int64
		_ = db.QueryRow("SELECT started_at, duration_sec FROM sessions").Scan(&startAt, &duration)
		
		y2, m2, d2 := now.Date()
		ds2 := time.Date(y2, m2, d2, 0, 0, 0, 0, now.Location()).Unix()
		de2 := time.Date(y2, m2, d2, 23, 59, 59, 0, now.Location()).Unix()
		
		t.Errorf("No stats found. DB count: %d, Row: %d (dur %d). Query Range: %d - %d", 
			count, startAt, duration, ds2, de2)
		return
	}

	assert.Len(t, stats, 1)
	assert.Equal(t, "TestApp", stats[0].AppName)
	assert.Equal(t, int64(60), stats[0].TotalSeconds)
}



func TestDeleteOldSessions(t *testing.T) {
	dbPath := "test_cleanup_final.db"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)

	db, err := Open(dbPath)
	assert.NoError(t, err)
	defer db.Close()


	err = Migrate(db)
	assert.NoError(t, err)

	now := time.Now()
	// Insert a very old session (7 months ago)
	oldTime := now.AddDate(0, -7, 0).Unix()
	id, _ := InsertSession(db, "OldApp", "old.exe", "Old", oldTime)
	_ = CloseSession(db, id, oldTime+10)

	// Insert a recent session
	recentTime := now.Unix()
	id2, _ := InsertSession(db, "NewApp", "new.exe", "New", recentTime)
	_ = CloseSession(db, id2, recentTime+10)

	err = DeleteOldSessions(db)
	assert.NoError(t, err)

	stats, _ := GetDailyStats(db, now)
	assert.Len(t, stats, 1)
	assert.Equal(t, "NewApp", stats[0].AppName)
}


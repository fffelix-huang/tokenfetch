package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

func insert(t *testing.T, st *Store, ev usage.Event) {
	t.Helper()
	tx, err := st.Begin()
	require.NoError(t, err)
	require.NoError(t, tx.Upsert(ev))
	require.NoError(t, tx.SetFileState("/log.jsonl", FileState{Offset: 1, Size: 1, MTime: time.Unix(1, 0)}))
	require.NoError(t, tx.Commit())
}

func count(t *testing.T, st *Store) int64 {
	t.Helper()
	groups, err := st.Groups(time.Unix(0, 0), time.Unix(1<<40, 0), 100_000)
	require.NoError(t, err)
	var n int64
	for _, g := range groups {
		n += g.Messages
	}
	return n
}

var ev = usage.Event{Provider: "p", MessageID: "m1", Timestamp: time.Unix(1_000_000, 0), Model: "claude-opus-5",
	Tokens: usage.Tokens{Output: 5}}

func TestForgetFilesKeepsEvents(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), fileName))
	require.NoError(t, err)
	defer st.Close()
	insert(t, st, ev)
	require.NoError(t, st.ForgetFiles())
	states, _ := st.FileStates()
	assert.Empty(t, states, "file states after ForgetFiles")
	assert.EqualValues(t, 1, count(t, st), "events after ForgetFiles")
}

func TestUpsertRefreshesAllColumns(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), fileName))
	require.NoError(t, err)
	defer st.Close()
	insert(t, st, ev)
	fixed := ev
	fixed.Skill = "tdd"
	insert(t, st, fixed)
	groups, _ := st.Groups(time.Unix(0, 0), time.Unix(1<<40, 0), 100_000)
	require.Len(t, groups, 1)
	assert.Equal(t, "tdd", groups[0].Skill, "re-read should refresh non-token columns")
}

func TestReopenKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	st, err := Open(path)
	require.NoError(t, err)
	insert(t, st, ev)
	st.Close()
	st, err = Open(path)
	require.NoError(t, err)
	defer st.Close()
	assert.EqualValues(t, 1, count(t, st), "events lost on reopen")
}

func TestNewerSchemaRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	st, err := Open(path)
	require.NoError(t, err)
	st.db.Exec("PRAGMA user_version = 99")
	st.Close()
	_, err = Open(path)
	assert.ErrorContains(t, err, "newer")
}

func TestMoveLegacy(t *testing.T) {
	old := filepath.Join(t.TempDir(), "Caches", "tokenfetch", fileName)
	st, err := Open(old)
	require.NoError(t, err)
	insert(t, st, ev) // leaves data in the WAL while open
	newPath := filepath.Join(t.TempDir(), "Application Support", "tokenfetch", fileName)
	moved, err := MoveLegacy(old, newPath)
	st.Close()
	require.NoError(t, err)
	require.True(t, moved)
	assert.NoDirExists(t, filepath.Dir(old), "empty legacy dir should be removed")
	st, err = Open(newPath)
	require.NoError(t, err)
	defer st.Close()
	assert.EqualValues(t, 1, count(t, st), "events lost in move")
	moved, _ = MoveLegacy(old, newPath)
	assert.False(t, moved, "second move should be a no-op")
}

// Requests over the long-prompt threshold group apart so they can be priced apart.
func TestGroupsSplitLongPrompt(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), fileName))
	require.NoError(t, err)
	defer st.Close()
	at, over, overByWrite := ev, ev, ev
	at.MessageID, at.Input, at.CacheRead = "a", 10, 99_990 // prompt 100_000: not over
	over.MessageID, over.Input, over.CacheRead = "b", 10, 100_000
	overByWrite.MessageID, overByWrite.CacheRead, overByWrite.CacheWrite1h = "c", 100_000, 1
	for _, e := range []usage.Event{at, over, overByWrite} {
		insert(t, st, e)
	}
	groups, err := st.Groups(time.Unix(0, 0), time.Unix(1<<40, 0), 100_000)
	require.NoError(t, err)
	byLong := map[bool]int64{}
	for _, g := range groups {
		byLong[g.LongPrompt] += g.Messages
	}
	assert.Equal(t, map[bool]int64{false: 1, true: 2}, byLong)
}

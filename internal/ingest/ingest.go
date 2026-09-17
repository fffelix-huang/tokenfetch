// Package ingest incrementally copies source logs into the store.
package ingest

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"

	"github.com/fffelix-huang/tokenfetch/internal/store"
	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

type Stats struct {
	FilesScanned int
	Events       int
	BadLines     int
}

// Run ingests appended bytes of every source file. Only newline-terminated
// lines are consumed, so a line being written is picked up next run.
// Events of deleted logs stay stored (Claude Code prunes old transcripts).
func Run(st *store.Store, sources []usage.Source) (Stats, error) {
	var stats Stats
	states, err := st.FileStates()
	if err != nil {
		return stats, err
	}
	tx, err := st.Begin()
	if err != nil {
		return stats, err
	}
	defer tx.Rollback()

	for _, src := range sources {
		files, err := src.Files()
		if err != nil {
			return stats, err
		}
		for _, path := range files {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			prev, seen := states[path]
			if seen && info.Size() == prev.Size && info.ModTime().Equal(prev.MTime) {
				continue
			}
			offset := prev.Offset
			if info.Size() < offset {
				offset = 0 // truncated or replaced: reparse; upserts are idempotent
			}
			newOffset, err := ingestFile(tx, src, path, offset, &stats)
			if err != nil {
				return stats, err
			}
			stats.FilesScanned++
			if err := tx.SetFileState(path, store.FileState{Offset: newOffset, Size: info.Size(), MTime: info.ModTime()}); err != nil {
				return stats, err
			}
		}
	}
	return stats, tx.Commit()
}

func ingestFile(tx *store.Tx, src usage.Source, path string, offset int64, stats *Stats) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return offset, nil // vanished between Stat and Open
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, err
	}
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		line, err := r.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			return offset, nil // drop unterminated tail
		}
		if err != nil {
			return offset, err
		}
		offset += int64(len(line))
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		ev, ok, err := src.ParseLine(line)
		if err != nil {
			stats.BadLines++
			continue
		}
		if !ok {
			continue
		}
		if err := tx.Upsert(ev); err != nil {
			return offset, err
		}
		stats.Events++
	}
}

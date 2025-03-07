// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: query.sql

package database

import (
	"context"
)

const getClosestHashes = `-- name: GetClosestHashes :many
SELECT songs.id, songs.name
FROM songs
JOIN song_hashes ON songs.id = song_hashes.song_id
WHERE song_hashes.song_hash BETWEEN ?1 - ?2 AND ?1 + ?2
`

type GetClosestHashesParams struct {
	TargetHash interface{}
	Tolerance  interface{}
}

func (q *Queries) GetClosestHashes(ctx context.Context, arg GetClosestHashesParams) ([]Song, error) {
	rows, err := q.db.QueryContext(ctx, getClosestHashes, arg.TargetHash, arg.Tolerance)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Song
	for rows.Next() {
		var i Song
		if err := rows.Scan(&i.ID, &i.Name); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getSongByHash = `-- name: GetSongByHash :many
SELECT songs.id, songs.name 
FROM songs 
JOIN song_hashes ON songs.id = song_hashes.song_id 
WHERE song_hashes.song_hash = ?
`

func (q *Queries) GetSongByHash(ctx context.Context, songHash int64) ([]Song, error) {
	rows, err := q.db.QueryContext(ctx, getSongByHash, songHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Song
	for rows.Next() {
		var i Song
		if err := rows.Scan(&i.ID, &i.Name); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getSongByID = `-- name: GetSongByID :one
SELECT id, name FROM songs WHERE id = ?
`

func (q *Queries) GetSongByID(ctx context.Context, id int64) (Song, error) {
	row := q.db.QueryRowContext(ctx, getSongByID, id)
	var i Song
	err := row.Scan(&i.ID, &i.Name)
	return i, err
}

const insertSong = `-- name: InsertSong :one
INSERT INTO songs (name) VALUES (?) RETURNING id
`

func (q *Queries) InsertSong(ctx context.Context, name string) (int64, error) {
	row := q.db.QueryRowContext(ctx, insertSong, name)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const insertSongHash = `-- name: InsertSongHash :exec
INSERT INTO song_hashes (song_id, song_hash) VALUES (?, ?)
`

type InsertSongHashParams struct {
	SongID   int64
	SongHash int64
}

func (q *Queries) InsertSongHash(ctx context.Context, arg InsertSongHashParams) error {
	_, err := q.db.ExecContext(ctx, insertSongHash, arg.SongID, arg.SongHash)
	return err
}

const removeSharedHashes = `-- name: RemoveSharedHashes :exec
DELETE FROM song_hashes
WHERE song_hash IN (
    SELECT song_hash
    FROM song_hashes
    GROUP BY song_hash
    HAVING COUNT(DISTINCT song_id) > 1
)
`

func (q *Queries) RemoveSharedHashes(ctx context.Context) error {
	_, err := q.db.ExecContext(ctx, removeSharedHashes)
	return err
}

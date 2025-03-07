-- name: InsertSong :one
INSERT INTO songs (name) VALUES (?) RETURNING id;

-- name: InsertSongHash :exec
INSERT INTO song_hashes (song_id, song_hash) VALUES (?, ?);

-- name: GetSongByID :one
SELECT id, name FROM songs WHERE id = ?;

-- name: GetSongByHash :many
SELECT songs.id, songs.name 
FROM songs 
JOIN song_hashes ON songs.id = song_hashes.song_id 
WHERE song_hashes.song_hash = ?;

-- name: GetClosestHashes :many
SELECT songs.id, songs.name
FROM songs
JOIN song_hashes ON songs.id = song_hashes.song_id
WHERE song_hashes.song_hash BETWEEN @target_hash - @tolerance AND @target_hash + @tolerance;

-- name: RemoveSharedHashes :exec
DELETE FROM song_hashes
WHERE song_hash IN (
    SELECT song_hash
    FROM song_hashes
    GROUP BY song_hash
    HAVING COUNT(DISTINCT song_id) > 1
);
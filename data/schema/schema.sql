
CREATE TABLE IF NOT EXISTS songs (
    id INTEGER PRIMARY KEY,
    name text NOT NULL
);

CREATE TABLE IF NOT EXISTS song_hashes (
    id INTEGER PRIMARY KEY,
    song_id INTEGER NOT NULL,
    song_hash INTEGER NOT NULL,

    FOREIGN KEY (song_id) REFERENCES songs (id)
);
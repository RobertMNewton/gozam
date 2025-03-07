package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	_ "modernc.org/sqlite"

	"github.com/RobertMNewton/gozam/internal/database"
	"github.com/RobertMNewton/gozam/pkg/fingerprint"
	"github.com/go-audio/wav"
)

var ddl string = `
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
`

var songs = map[string]string{
	"Baby Shark Dance":                              "XqZsoesa55w",
	"Despacito by Luis Fonsi ft. Daddy Yankee":      "kJQP7kiw5Fk",
	"Shape of You by Ed Sheeran":                    "JGwWNGJdvx8",
	"See You Again by Wiz Khalifa ft. Charlie Puth": "RgKAFK5djSk",
	"Uptown Funk by Mark Ronson ft. Bruno Mars":     "OPf0YbXqDm0",
	"Gangnam Style by PSY":                          "cGc_NfiTxng",
	"Roar by Katy Perry":                            "CevxZvSJLk8",
	"Perfect by Ed Sheeran":                         "2Vv-BfVoq4g",
	"Girls Like You by Maroon 5 ft. Cardi B":        "aJOTlE1K90k",
	"Faded by Alan Walker":                          "60ItHLz5WEA",
	"Let Her Go by Passenger":                       "RBumgq5yVrA",
	"Thinking Out Loud by Ed Sheeran":               "LPn0KFlbqX8",
	"I'm on a Boat by The Lonely Island ft. T-Pain": "2zNSgSzhBfM",
}

const (
	BIN_SIZE           int = 44800 / 10
	OVERLAP            int = 44800 / 4 / 10
	HASH_TOP_N         int = 1000
	MAX_TOKEN_TIME_DFF int = 25
)

func main() {
	ctx := context.Background()

	db, err := sql.Open("sqlite", "data/gozam.db")
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if _, err := db.ExecContext(ctx, ddl); err != nil {
		log.Fatalf("failed to create to tables in database: %v", err)
	}

	queries := database.New(db)

	for songName, ytID := range songs {
		songID, err := queries.InsertSong(ctx, songName)
		if err != nil {
			log.Fatalf("failed to insert song '%s' into db: %v", songName, err)
		}

		filepath := fmt.Sprintf("data/tmp/%s", ytID)
		if !fileExists(filepath) {
			saveYoutubeAudio(ytID, filepath)
		}

		reader, err := decodeWebMAudioToPCMReader(ytID)
		if err != nil {
			fmt.Printf("Error decoding audio for '%s': %v\n", ytID, err)
			continue
		}

		decoder := wav.NewDecoder(reader)
		buff, err := decoder.FullPCMBuffer()
		if err != nil {
			log.Fatalf("failed to decode file '%s': %v", filepath, err)
		}

		fmt.Printf("hashing song %s... \n", songName)
		songFingerprint := fingerprint.GetFingerPrint2(buff, BIN_SIZE, OVERLAP, HASH_TOP_N, MAX_TOKEN_TIME_DFF, 3)
		for hash := range songFingerprint.Hashes {
			err := queries.InsertSongHash(ctx, database.InsertSongHashParams{
				SongID:   songID,
				SongHash: int64(hash),
			})
			if err != nil {
				log.Fatalf("failed to insert song hash: %v", err)
			}
		}
	}
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func decodeWebMAudioToPCMReader(ytID string) (io.ReadSeeker, error) {
	filepath := fmt.Sprintf("data/tmp/%s", ytID)
	wavFilepath := fmt.Sprintf("data/tmp/%s.wav", ytID)

	if !fileExists(wavFilepath) {
		if !fileExists(filepath) {
			if err := saveYoutubeAudio(ytID, filepath); err != nil {
				return nil, fmt.Errorf("failed to download YouTube audio: %w", err)
			}
		}

		// Convert WebM to WAV using ffmpeg
		cmd := exec.Command("ffmpeg", "-i", filepath, "-f", "wav", wavFilepath)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to convert to WAV with ffmpeg: %w", err)
		}
	}

	// Open the WAV file
	file, err := os.Open(wavFilepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAV file: %w", err)
	}

	return file, nil
}

func saveYoutubeAudio(ytID string, outputFile string) error {
	url := fmt.Sprintf("https://www.youtube.com/watch?v=%s", ytID)
	cmd := exec.Command("yt-dlp", "-f", "bestaudio", "-o", outputFile, url)
	out, err := cmd.CombinedOutput()
	fmt.Printf("%s\n", out)
	return err
}

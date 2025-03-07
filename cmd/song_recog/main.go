package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"

	_ "modernc.org/sqlite"

	"github.com/RobertMNewton/gozam/internal/database"
	"github.com/RobertMNewton/gozam/pkg/fingerprint"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gordonklaus/portaudio"
)

const (
	BIN_SIZE           int = 44800 / 10
	OVERLAP            int = 44800 / 4 / 10
	HASH_TOP_N         int = 1000
	MAX_TOKEN_TIME_DFF int = 25
	SAMPLE_RATE        int = 44800 // Standard audio sample rate
	BUFFER_SIZE        int = 4096  // Buffer size for capturing audio
	DURATION           int = 10    // Record for 10 seconds
)

func main() {
	ctx := context.Background()

	// Connect to database
	db, err := sql.Open("sqlite", "data/gozam.db")
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	queries := database.New(db)

	// Initialize PortAudio
	portaudio.Initialize()
	defer portaudio.Terminate()

	// Start capturing microphone input
	fmt.Println("Recording audio for song recognition...")
	buffer, err := recordAudio(DURATION)
	if err != nil {
		log.Fatalf("failed to record audio: %v", err)
	}

	err = saveAudioBufferToFile("debug_recording.wav", buffer, SAMPLE_RATE)
	if err != nil {
		log.Fatalf("failed to save audio: %v", err)
	}

	intBuffer := make([]int, len(buffer))
	for i, x := range buffer {
		intBuffer[i] = int(x)
	}

	// Convert captured data into an audio buffer
	audioBuffer := &audio.IntBuffer{
		Data:           intBuffer,
		Format:         &audio.Format{SampleRate: SAMPLE_RATE, NumChannels: 1},
		SourceBitDepth: 16,
	}

	// Generate fingerprint from recorded audio
	songFingerprint := fingerprint.GetFingerPrint2(audioBuffer, BIN_SIZE, OVERLAP, HASH_TOP_N, MAX_TOKEN_TIME_DFF, 3)

	// Try to find a match in the database
	matchedSong, err := findMatchingSong(ctx, queries, songFingerprint)
	if err != nil {
		log.Fatalf("error searching for matching song: %v", err)
	}

	if len(matchedSong) != 0 {
		songs := make([]struct {
			string
			float32
		}, len(matchedSong))
		for songName, perc := range matchedSong {
			songs = append(songs, struct {
				string
				float32
			}{songName, perc})
		}

		sort.Slice(songs, func(i, j int) bool {
			return songs[i].float32 > songs[j].float32
		})

		for _, song := range songs[:3] {
			fmt.Printf("Song: '%s', Match: %f\n", song.string, song.float32*100)
		}
	} else {
		fmt.Println("No matching song found with normal hash... using closest hash algo now")
	}
}

func recordAudio(seconds int) ([]int16, error) {
	// Initialize PortAudio
	err := portaudio.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PortAudio: %v", err)
	}
	defer portaudio.Terminate()

	// Create input buffer for PortAudio to write into
	tempBuffer := make([]int16, BUFFER_SIZE)

	// Open stream with input buffer reference
	stream, err := portaudio.OpenDefaultStream(
		1, // num input channels
		0, // num output channels
		float64(SAMPLE_RATE),
		BUFFER_SIZE,
		&tempBuffer, // input buffer pointer
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %v", err)
	}
	defer stream.Close()

	// Main buffer to store entire recording
	mainBuffer := make([]int16, SAMPLE_RATE*seconds)

	// Handle interrupts
	stop := make(chan struct{})
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		close(stop)
	}()

	// Start recording
	err = stream.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start stream: %v", err)
	}
	defer stream.Stop()

	// Record audio in chunks
	index := 0
	for index < len(mainBuffer) {
		select {
		case <-stop:
			return mainBuffer[:index], nil
		default:
			err := stream.Read()
			if err != nil {
				return nil, fmt.Errorf("read error: %v", err)
			}

			// Copy from temp buffer to main buffer
			n := copy(mainBuffer[index:], tempBuffer)
			index += n
		}
	}

	return mainBuffer, nil
}

// findMatchingSong queries the database for a song with the most matching hashes
func findMatchingSong(ctx context.Context, queries *database.Queries, fingerprint fingerprint.Fingerprint) (map[string]float32, error) {
	matchCounts := make(map[int64]int)

	// Query for each hash in the fingerprint
	for hash := range fingerprint.Hashes {
		songHashes, err := queries.GetSongByHash(ctx, int64(hash))
		if err != nil {
			return nil, fmt.Errorf("failed to query song hashes: %w", err)
		}

		// Count matches per song
		for _, song := range songHashes {
			matchCounts[song.ID]++
		}
	}

	res, total := make(map[string]float32), 0
	for songID, count := range matchCounts {
		song, err := queries.GetSongByID(ctx, songID)
		if err != nil {
			continue
		}

		res[song.Name] = float32(count)
		total += count
	}

	for song := range res {
		res[song] /= float32(total)
	}

	return res, nil
}

func findMatchingSongClosest(ctx context.Context, queries *database.Queries, fingerprint fingerprint.Fingerprint, tolerance int64) (string, error) {
	matchCounts := make(map[int64]int)

	// Query for each hash in the fingerprint
	for hash := range fingerprint.Hashes {
		// Find closest hashes within the specified tolerance
		closestHashes, err := queries.GetClosestHashes(ctx, database.GetClosestHashesParams{
			TargetHash: int64(hash),
			Tolerance:  tolerance,
		})
		if err != nil {
			return "", fmt.Errorf("failed to query closest hashes: %w", err)
		}

		// Count matches per song
		for _, song := range closestHashes {
			matchCounts[song.ID]++
		}
	}

	// Find song with the most matches
	var bestMatchID int64
	maxMatches := 0
	for songID, count := range matchCounts {
		if count > maxMatches {
			bestMatchID = songID
			maxMatches = count
		}
	}

	// If no matches found, return empty
	if maxMatches == 0 {
		return "", nil
	}

	// Get song name by ID
	song, err := queries.GetSongByID(ctx, bestMatchID)
	if err != nil {
		return "", fmt.Errorf("failed to get song name: %w", err)
	}

	return song.Name, nil
}

func saveAudioBufferToFile(filename string, buffer []int16, sampleRate int) error {
	outFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encoder := wav.NewEncoder(outFile, sampleRate, 16, 1, 1)

	// Convert int16 to int slice with proper scaling
	intBuffer := make([]int, len(buffer))
	for i, v := range buffer {
		intBuffer[i] = int(v)
	}

	audioBuffer := &audio.IntBuffer{
		Data:           intBuffer,
		Format:         &audio.Format{SampleRate: sampleRate, NumChannels: 1},
		SourceBitDepth: 16,
	}

	if err := encoder.Write(audioBuffer); err != nil {
		return err
	}
	return encoder.Close()
}

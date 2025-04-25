package function

import (
	"bytes"
	"fmt"
	"karaoke_generator/progress"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Config struct {
	InputAudioFile string
	InputLyricsSrc string
	OutputDir      string
	Filename       string
	SessionID      string
	language       int
}

type Pair[T any] struct {
	dictionary T
	acoustic   T
}

var dic = map[int]Pair[string]{
	1: {dictionary: "vietnamese_mfa", acoustic: "vietnamese_mfa"},
	2: {dictionary: "english_us_mfa", acoustic: "english_mfa"},
}

// Sử dụng file và lyrics từ người dùng
func GenerateKaraokeFromUpload(audioPath string, lyricsContent string, sessionID string, language int) error {
	// Tạo config với đường dẫn file từ người dùng
	config := Config{
		InputLyricsSrc: "./function/input/vocals_48k.lab",
		InputAudioFile: audioPath,
		OutputDir:      "./function/output",
		SessionID:      sessionID,
		language:       language,
	}

	// Extract filename without extension
	config.Filename = strings.TrimSuffix(filepath.Base(config.InputAudioFile), filepath.Ext(config.InputAudioFile))

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return fmt.Errorf("Error creating output directory: %v", err)
	}

	// Execute the karaoke generation pipeline
	if err := generateKaraokeReal(config); err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	return nil
}

// GenerateKaraoke creates a karaoke track from an audio file
func generateKaraokeReal(config Config) error {
	// Step 1: Run Demucs to separate vocals and music
	progress.UpdateProgress(config.SessionID, 10, "Demucs processing", "Demucs processing")
	if err := runDemucs(config); err != nil {

		return fmt.Errorf("demucs processing failed: %w", err)
	}
	progress.UpdateProgress(config.SessionID, 20, "Demucs processing completed", "Demucs processing completed")

	// Step 2: Convert WAV files to 48kHz
	if err := convertTo48kHz(config); err != nil {
		return fmt.Errorf("48kHz conversion failed: %w", err)
	}

	//progress.UpdateProgress(config.SessionID, 30, "48kHz conversion completed", "48kHz conversion completed")

	// Step 3: Convert WAV files to OGG format
	if err := convertToOgg(config); err != nil {
		return fmt.Errorf("OGG conversion failed: %w", err)
	}

	progress.UpdateProgress(config.SessionID, 40, "OGG conversion completed", "OGG conversion completed")

	// // Step 4: Move OGG files to output directory
	// if err := moveOggFiles(config); err != nil {
	// 	return fmt.Errorf("moving OGG files failed: %w", err)aa
	// }

	progress.UpdateProgress(config.SessionID, 50, "OGG files moved to output directory", "OGG files moved to output directory")

	// Step 5: Generate timestamp file
	if err := generateTimestamps(config); err != nil {
		fmt.Println("ERROR timestamp generation failed: %w", err)
		return fmt.Errorf("timestamp generation failed: %w", err)
	}

	progress.UpdateProgress(config.SessionID, 60, "Timestamp file generated", "Timestamp file generated")

	if err := archiveAllAssests(config); err != nil {
		return fmt.Errorf("archive all assets failed: %w", err)
	}

	progress.UpdateProgress(config.SessionID, 100, "Final files generated", "Final files generated")
	return nil
}

func archiveAllAssests(config Config) error {

	ogg_vocal := filepath.Join(config.OutputDir, "htdemucs", config.Filename, "vocals_48k_48k.ogg")
	ogg_no_vocal := filepath.Join(config.OutputDir, "htdemucs", config.Filename, "no_vocals_48k_48k.ogg")
	timestamp_output := filepath.Join("./function/timestamp_output/output_with_notes.json")
	fmt.Println(ogg_vocal)
	fmt.Println(ogg_no_vocal)
	fmt.Println(timestamp_output)

	if err := os.Rename(ogg_vocal, "./function/final_result/vocal_48k.ogg"); err != nil {
		fmt.Println("error moving vocals OGG: %w", err)
		return fmt.Errorf("error moving vocals OGG: %w", err)
	}

	if err := os.Rename(ogg_no_vocal, "./function/final_result/no_vocals_48k.ogg"); err != nil {
		fmt.Println("error moving no vocals OGG: %w", err)
		return fmt.Errorf("error moving no vocals OGG: %w", err)
	}

	if err := os.Rename(timestamp_output, "./function/final_result/timestamp_with_notes.json"); err != nil {
		fmt.Println("error moving timestamp output: %w", err)
		return fmt.Errorf("error moving timestamp output: %w", err)
	}

	return nil
}

func runDemucs(config Config) error {
	fmt.Println("Running demucs on:", config.InputAudioFile)
	fmt.Println("Output will be saved to:", config.OutputDir)

	cmd := exec.Command("bash", "-c", fmt.Sprintf(
		"/Users/mac/miniconda3/bin/conda run -n demucs_env demucs --two-stems=vocals --out=\"%s\" \"%s\"",
		config.OutputDir, config.InputAudioFile))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func convertTo48kHz(config Config) error {
	// Set file paths
	vocalsFile := filepath.Join(config.OutputDir, "htdemucs", config.Filename, "vocals.wav")
	vocals48k := filepath.Join(config.OutputDir, "htdemucs", config.Filename, "vocals_48k.wav")
	noVocalsFile := filepath.Join(config.OutputDir, "htdemucs", config.Filename, "no_vocals.wav")
	noVocals48k := filepath.Join(config.OutputDir, "htdemucs", config.Filename, "no_vocals_48k.wav")

	// Convert vocals WAV from 44.1kHz to 48kHz
	fmt.Println("Converting vocals from 44.1kHz to 48kHz...")
	cmd := exec.Command("ffmpeg", "-i", vocalsFile, "-ar", "48000", vocals48k)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error converting vocals to 48kHz: %w", err)
	}

	// Convert no_vocals WAV from 44.1kHz to 48kHz
	fmt.Println("Converting no_vocals from 44.1kHz to 48kHz...")
	cmd = exec.Command("ffmpeg", "-i", noVocalsFile, "-ar", "48000", noVocals48k)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func convertToOgg(config Config) error {

	// Convert vocals 48k WAV to OGG
	fmt.Println("Converting vocals to OGG format...")
	cmd := exec.Command("./function/ogg/wav2ogg", "-i", filepath.Join(config.OutputDir, "htdemucs", config.Filename, "no_vocals_48k.wav"), "-b", "48k", "-m")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error converting vocals to OGG: %w", err)
	}

	// Convert no_vocals 48k WAV to OGG
	fmt.Println("Converting no_vocals to OGG format...")
	cmd = exec.Command("./function/ogg/wav2ogg", "-i", filepath.Join(config.OutputDir, "htdemucs", config.Filename, "vocals_48k.wav"), "-b", "48k", "-m")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func generateTimestamps(config Config) error {
	// Change to MFA directory
	fmt.Println("Generating timestamp file...")
	fmt.Println(config.Filename)

	// Check if input file exists
	vocalsSrc := filepath.Join(config.OutputDir, "htdemucs", config.Filename, "vocals_48k.wav")
	if _, err := os.Stat(vocalsSrc); os.IsNotExist(err) {
		return fmt.Errorf("vocals file not found at %s: %w", vocalsSrc, err)
	}

	// Move files to input_files directory
	vocalsDest := filepath.Join("./function/input", fmt.Sprintf("%s.wav", config.Filename))

	// Create input directory if it doesn't exist
	if err := os.MkdirAll("./function/input", 0755); err != nil {
		return fmt.Errorf("error creating input directory: %w", err)
	}

	// If destination file already exists, remove it
	if _, err := os.Stat(vocalsDest); err == nil {
		if err := os.Remove(vocalsDest); err != nil {
			return fmt.Errorf("error removing existing destination file: %w", err)
		}
	}

	if err := os.Rename(vocalsSrc, vocalsDest); err != nil {
		return fmt.Errorf("error moving vocals input_files: %w", err)
	}

	// Check if MP3 file exists before attempting to remove it
	mp3Path := filepath.Join("./function/input", fmt.Sprintf("%s.mp3", config.Filename))
	if _, err := os.Stat(mp3Path); err == nil {
		if err := os.Remove(mp3Path); err != nil {
			return fmt.Errorf("error deleting vocals input_files: %w", err)
		}
	}

	// Dynamically determine conda path instead of hardcoding
	condaBasePath := "/home/user/miniconda3"
	// Check common locations for conda
	condaPaths := []string{
		"/home/user/miniconda3/bin/conda",
		"/Users/mac/miniconda3/bin/conda", // MacOS path
		"/usr/local/miniconda3/bin/conda",
		os.ExpandEnv("$HOME/miniconda3/bin/conda"),
	}

	var condaPath string
	for _, path := range condaPaths {
		if _, err := os.Stat(path); err == nil {
			condaPath = path
			condaBasePath = filepath.Dir(filepath.Dir(path))
			break
		}
	}

	if condaPath == "" {
		return fmt.Errorf("conda executable not found in common locations")
	}
	fmt.Println("Using conda at:", condaPath)

	// Check if mfa environment exists
	cmd := exec.Command(condaPath, "env", "list")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to list conda environments: %w", err)
	}
	if !strings.Contains(out.String(), "mfa") {
		return fmt.Errorf("conda environment 'mfa' not found, please create it first")
	}

	// Check if mandarin_mfa dictionary and acoustic model exist
	checkCmd := fmt.Sprintf(
		"source %q/etc/profile.d/conda.sh && conda activate mfa && mfa models list dictionary",
		condaBasePath,
	)
	cmd = exec.Command("bash", "-c", checkCmd)
	var checkOut, checkErr bytes.Buffer
	cmd.Stdout = &checkOut
	cmd.Stderr = &checkErr
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s/bin:%s", condaBasePath, os.Getenv("PATH")),
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to check MFA models: %w\nStderr: %s", err, checkErr.String())
	}
	//mandarin_mfa
	//mandarin_mfa
	// If mandarin_mfa is not found, download it
	if !strings.Contains(checkOut.String(), "mandarin_mfa") {
		fmt.Println("mandarin_mfa not found, downloading models...")
		downloadCmd := fmt.Sprintf(
			"source %q/etc/profile.d/conda.sh && conda activate mfa && mfa model download dictionary mandarin_mfa && mfa model download acoustic mandarin_mfa",
			condaBasePath,
		)
		cmd = exec.Command("bash", "-c", downloadCmd)
		var dlOut, dlErr bytes.Buffer
		cmd.Stdout = &dlOut
		cmd.Stderr = &dlErr
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("PATH=%s/bin:%s", condaBasePath, os.Getenv("PATH")),
			fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
		)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to download mandarin_mfa models: %w\nStdout: %s\nStderr: %s", err, dlOut.String(), dlErr.String())
		}
		fmt.Println("mandarin_mfa models downloaded successfully")
	}

	// Check if input directory has files
	entries, err := os.ReadDir("./function/input")
	if err != nil {
		return fmt.Errorf("error reading input directory: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("input directory is empty, no files to process")
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll("./function/output", 0755); err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}
	fmt.Printf(
		"source %q/etc/profile.d/conda.sh && conda activate mfa && mfa models list dictionary && mfa align ./function/input %s %s ./function/timestamp_output --beam 100 --retry_beam 400 --clean\n",
		condaBasePath,
		dic[config.language].dictionary,
		dic[config.language].acoustic,
	)

	// Prepare the MFA command with debug output
	mfaCmd := fmt.Sprintf(
		"source %q/etc/profile.d/conda.sh && conda activate mfa && mfa models list dictionary && mfa align ./function/input %s %s ./function/timestamp_output --beam 100 --retry_beam 400 --clean",
		condaBasePath,
		dic[config.language].dictionary,
		dic[config.language].acoustic,
	)

	// Execute the command in a bash shell
	cmd = exec.Command("bash", "-c", mfaCmd)

	// Set environment to include conda bin directory and HOME
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home/user"
	}
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s/bin:%s", condaBasePath, os.Getenv("PATH")),
		fmt.Sprintf("HOME=%s", homeDir),
	)

	// Capture both stdout and stderr for better debugging
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run MFA align: %w\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Verify output files were created
	outputFiles, err := os.ReadDir("./function/output")
	if err != nil {
		return fmt.Errorf("error reading output directory: %w", err)
	}
	if len(outputFiles) == 0 {
		return fmt.Errorf("MFA processing completed but no output files were generated")
	}

	fmt.Println("MFA align completed successfully")
	fmt.Println("Output:", stdout.String())

	if err := TextGridToJSON(
		filepath.Join("./function/timestamp_output", fmt.Sprintf("%s.TextGrid", config.Filename)),
		filepath.Join("./function/input", fmt.Sprintf("%s.lab", config.Filename)),
		filepath.Join("./function/timestamp_output", "output.json"),
	); err != nil {
		return fmt.Errorf("error converting TextGrid to JSON: %w", err)
	}

	pythonScriptSrc := filepath.Join("./function/vocal_pitch_analyzer.py")

	cmd = exec.Command("bash", "-c", fmt.Sprintf(
		"python %s %s %s --output %s --log %s --quiet",
		pythonScriptSrc,
		filepath.Join("./function/timestamp_output", "output.json"),
		vocalsDest,
		filepath.Join("./function/timestamp_output", "output_with_notes.json"),
		filepath.Join("./function/timestamp_output", "pitch_analysis_log.json")))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running vocal_pitch_analyzer.py: %w", err)
	}

	return nil
}

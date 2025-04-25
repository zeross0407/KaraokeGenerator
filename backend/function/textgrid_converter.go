package function

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Interval represents a time interval with start, end and a label
type Interval struct {
	Start float64
	End   float64
	Label string
}

// WordInfo represents information about a word in the lyrics
type WordInfo struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Segment represents a line of text with timing information
type Segment struct {
	Start float64    `json:"start"`
	End   float64    `json:"end"`
	Text  string     `json:"text"`
	Words []WordInfo `json:"words"`
}

// LyricsJSON represents the final JSON structure
type LyricsJSON struct {
	Text     string    `json:"text"`
	Segments []Segment `json:"segments"`
	Language string    `json:"language"`
}

// parseTextGrid parses a TextGrid file and extracts the word intervals
func parseTextGrid(filePath string) ([]Interval, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading TextGrid file: %w", err)
	}

	// Convert content to string
	text := string(content)

	// Find the "words" tier
	wordsPattern := regexp.MustCompile(`item \[\d+\]:\s+class = "IntervalTier"\s+name = "words"`)
	wordsMatch := wordsPattern.FindStringIndex(text)
	if wordsMatch == nil {
		return nil, fmt.Errorf("words tier not found in TextGrid file")
	}

	// Extract the intervals section for the words tier
	text = text[wordsMatch[1]:]

	// Find where the next tier begins or the file ends
	nextTierPattern := regexp.MustCompile(`item \[\d+\]:`)
	nextTierMatch := nextTierPattern.FindStringIndex(text)
	if nextTierMatch != nil {
		text = text[:nextTierMatch[0]]
	}

	// Parse intervals
	intervalPattern := regexp.MustCompile(`intervals \[\d+\]:\s+xmin = ([\d\.]+)\s+xmax = ([\d\.]+)\s+text = "(.*?)"`)
	intervalMatches := intervalPattern.FindAllStringSubmatch(text, -1)

	var intervals []Interval
	for _, match := range intervalMatches {
		if len(match) != 4 {
			continue
		}

		start, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing interval start time: %w", err)
		}

		end, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing interval end time: %w", err)
		}

		label := match[3]

		// Only include non-empty labels
		if label != "" {
			intervals = append(intervals, Interval{Start: start, End: end, Label: label})
		}
	}

	return intervals, nil
}

// readLabFile reads a lab file and returns its content as lines
func readLabFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening lab file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading lab file: %w", err)
	}

	return lines, nil
}

// TextGridToJSON converts a TextGrid file to JSON format based on a lab file
func TextGridToJSON(textgridPath, labPath, outputPath string) error {
	// Check if input files exist
	if _, err := os.Stat(textgridPath); os.IsNotExist(err) {
		return fmt.Errorf("TextGrid file does not exist: %s", textgridPath)
	}

	if _, err := os.Stat(labPath); os.IsNotExist(err) {
		return fmt.Errorf("Lab file does not exist: %s", labPath)
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}

	// Parse the TextGrid file
	wordIntervals, err := parseTextGrid(textgridPath)
	if err != nil {
		return fmt.Errorf("error parsing TextGrid: %w", err)
	}

	// Read the lab file
	labLines, err := readLabFile(labPath)
	if err != nil {
		return fmt.Errorf("error reading lab file: %w", err)
	}

	// Join all lines to create full text
	fullText := strings.Join(labLines, " ")

	// Process each line to extract words with their positions
	var lineWords [][]string
	for _, line := range labLines {
		words := strings.Fields(line)
		var formattedWords []string
		if len(words) > 0 {
			formattedWords = append(formattedWords, words[0]) // First word without space
			for i := 1; i < len(words); i++ {
				formattedWords = append(formattedWords, " "+words[i]) // Add space before words
			}
		}
		lineWords = append(lineWords, formattedWords)
	}

	// Flatten the array of words
	var allWords []string
	for _, words := range lineWords {
		allWords = append(allWords, words...)
	}

	// Map lines to word indices
	lineToWords := make(map[int][2]int)
	wordIndex := 0
	for i, words := range lineWords {
		lineToWords[i] = [2]int{wordIndex, wordIndex + len(words) - 1}
		wordIndex += len(words)
	}

	// Create segments based on lines
	var segments []Segment
	for lineIndex, line := range labLines {
		wordRange, ok := lineToWords[lineIndex]
		if !ok || wordRange[0] >= len(wordIntervals) {
			continue
		}

		// Ensure end index is within bounds
		endIndex := min(wordRange[1], len(wordIntervals)-1)

		segment := Segment{
			Start: round(wordIntervals[wordRange[0]].Start, 2),
			End:   round(wordIntervals[endIndex].End, 2),
			Text:  line,
			Words: []WordInfo{},
		}

		// Add word timing information
		for i := wordRange[0]; i <= endIndex; i++ {
			if i < len(allWords) && i < len(wordIntervals) {
				word := WordInfo{
					Word:  allWords[i],
					Start: round(wordIntervals[i].Start, 2),
					End:   round(wordIntervals[i].End, 2),
				}
				segment.Words = append(segment.Words, word)
			}
		}

		segments = append(segments, segment)
	}

	// Create the final JSON structure
	result := LyricsJSON{
		Text:     fullText,
		Segments: segments,
		Language: "vi",
	}

	// Write the result to file
	jsonData, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	err = ioutil.WriteFile(outputPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing JSON file: %w", err)
	}

	fmt.Printf("Successfully converted TextGrid to JSON: %s\n", outputPath)
	return nil
}

// Helper function to round float to specified decimal places
func round(num float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return math.Round(num*shift) / shift
}

// Helper function for finding the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

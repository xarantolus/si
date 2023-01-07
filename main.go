package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

var (
	//go:embed gifs
	gifs embed.FS
)

func writeSubtitle(text string, outPath string) (err error) {
	// Subtitles for 1h
	var str = `1
00:00:00,000 --> 01:00:00,000
` + text + "\n"

	return os.WriteFile(outPath, []byte(str), 0600)
}

type options struct {
	FontSize  int `json:"font_size"`
	Alignment int `json:"alignment"`
}

func createGif(gifPath string, text string, o options, outPath string) (err error) {
	var infoPath = strings.TrimSuffix(gifPath, path.Ext(gifPath)) + ".json"

	var gifOptions options

	content, err := gifs.ReadFile(infoPath)
	if err == nil {
		_ = json.Unmarshal(content, &gifOptions)
	}

	// Sensible default options
	if gifOptions.FontSize == 0 {
		gifOptions.FontSize = 50
	}
	if gifOptions.Alignment == 0 {
		gifOptions.Alignment = 6
	}

	// Whatever the user wants
	if o.FontSize != 0 {
		gifOptions.FontSize = o.FontSize
	}
	if o.Alignment != 0 {
		gifOptions.Alignment = o.Alignment
	}

	tmpDir, err := os.MkdirTemp("", "skill-issue")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		if tmpDir == "" {
			return
		}
		rmerr := os.RemoveAll(tmpDir)
		if err == nil {
			err = rmerr
		}
	}()

	const subtitleName = "sub.srt"
	err = writeSubtitle(text, path.Join(tmpDir, subtitleName))
	if err != nil {
		return fmt.Errorf("failed to write subtitle: %w", err)
	}

	// copy gif from embedded fs to temp dir
	gifBytes, err := fs.ReadFile(gifs, gifPath)
	if err != nil {
		return fmt.Errorf("failed to extract gif: %w", err)
	}

	gifDiskPath := filepath.Join(tmpDir, path.Base(gifPath))
	err = os.WriteFile(gifDiskPath, gifBytes, 0600)
	if err != nil {
		return fmt.Errorf("failed to write gif: %w", err)
	}

	firstGIFPath := filepath.Join(tmpDir, "first-"+path.Base(gifPath)+".mp4")
	var cmd = exec.Command(
		"ffmpeg",
		"-loglevel", "error",
		"-i", gifDiskPath,
		"-filter_complex", fmt.Sprintf("subtitles=%s:force_style='Fontname=Impact,Fontsize=%d,Alignment=%d'", subtitleName, gifOptions.FontSize, gifOptions.Alignment),
		firstGIFPath,
	)
	cmd.Dir = tmpDir

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run ffmpeg: %w", err)
	}

	// Now generate a palette for the gif
	var palettePath = filepath.Join(tmpDir, "palette.png")
	cmd = exec.Command(
		"ffmpeg",
		"-loglevel", "error",
		"-i", firstGIFPath,
		"-vf", "palettegen",
		palettePath,
	)
	cmd.Dir = tmpDir

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate palette: %w", err)
	}

	// Now generate the final gif
	cmd = exec.Command(
		"ffmpeg",
		"-loglevel", "error",
		"-i", firstGIFPath,
		"-i", palettePath,
		"-filter_complex", "paletteuse",
		outPath,
	)
	cmd.Dir = tmpDir

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to generate final gif: %w", err)
	}

	return nil
}

func listAvailableGIFs() (paths []string, err error) {
	var gifPaths []string

	err = fs.WalkDir(gifs, "gifs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && !strings.HasSuffix(d.Name(), ".json") {
			gifPaths = append(gifPaths, path)
		}

		return nil
	})

	return gifPaths, err
}

func main() {
	var (
		outputGIF string
		text      = strings.Join(os.Args[1:], " ")
	)

	inputText := strings.Join(strings.Fields(text), " ")
	if inputText == "" {
		inputText = "Skill Issue"
	}

	// Join all ascii chars into a filename and replace spaces with underscores
	outputGIF = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return '_'
		}
		if 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' {
			return r
		}
		return -1
	}, inputText) + ".gif"

	gifPaths, err := listAvailableGIFs()
	if err != nil {
		log.Fatalln("failed to list embedded gifs:", err)
	}

	// Select a random gif
	rand.Seed(time.Now().UnixNano())
	var randomGIFPath = gifPaths[rand.Intn(len(gifPaths))]

	outPath, err := filepath.Abs(outputGIF)
	if err != nil {
		log.Fatalln("failed to get absolute path:", err)
	}

	err = createGif(randomGIFPath, inputText, options{}, outPath)
	if err != nil {
		log.Fatalln("failed to create gif:", err)
	}

	log.Println("created gif:", outPath)
}

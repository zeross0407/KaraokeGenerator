package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"

	// Import local package cần sử dụng đường dẫn tương đối từ module root

	"karaoke_generator/function"
	"karaoke_generator/progress"
)

type LyricsRequest struct {
	Lyrics   string `json:"lyrics"`
	Language string `json:"language"`
}

// ProcessMessage là struct để gửi cập nhật tiến trình qua WebSocket
type ProcessMessage struct {
	Type              string  `json:"type"`
	Status            string  `json:"status"`
	Message           string  `json:"message"`
	Percentage        float64 `json:"percentage"`
	CurrentStep       string  `json:"current_step"`
	EstimatedTimeLeft string  `json:"estimated_time_left"`
	SessionID         string  `json:"sessionId"`
	Timestamp         int64   `json:"timestamp"`
}

// zipDirectory compresses a directory into a zip file
func zipDirectory(sourceDir, zipPath string) error {
	// Create a new zip file
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	// Create a new zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk through all files in the source directory
	err = filepath.Walk(sourceDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Create a local file header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Set compression method
		header.Method = zip.Deflate

		// Set relative path as header name
		relPath, err := filepath.Rel(sourceDir, filePath)
		if err != nil {
			return err
		}
		header.Name = relPath

		// Create writer for the file
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// Open source file
		srcFile, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		// Copy file content to the zip
		_, err = io.Copy(writer, srcFile)
		return err
	})

	return err
}

func main() {
	app := iris.New()

	// Allow OPTIONS method for CORS preflight requests
	app.AllowMethods(iris.MethodOptions)

	// Configure CORS headers in each handler
	app.Use(func(ctx iris.Context) {
		// Allow access from any origin
		ctx.Header("Access-Control-Allow-Origin", "*")

		// Allow all common HTTP methods
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")

		// Allow common headers and custom headers
		ctx.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization, X-Custom-Header")

		// Allow credentials (cookies, authorization headers, etc.)
		ctx.Header("Access-Control-Allow-Credentials", "true")

		// Cache preflight requests for 1 hour (3600 seconds)
		ctx.Header("Access-Control-Max-Age", "3600")

		// Allow browsers to expose these headers to JavaScript
		ctx.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Content-Disposition")

		// Handle preflight requests
		if ctx.Method() == iris.MethodOptions {
			ctx.StatusCode(iris.StatusOK)
			return
		}

		ctx.Next()
	})

	// Set up upload directory
	uploadDir := "./uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.Mkdir(uploadDir, 0755)
	}

	// Hello world endpoint
	app.Get("/api/hello", func(ctx iris.Context) {
		ctx.JSON(iris.Map{
			"message": "Hello World!",
			"status":  "success",
		})
	})

	// API endpoint để lấy tiến trình
	app.Get("/api/progress/{sessionID}", func(ctx iris.Context) {
		sessionID := ctx.Params().Get("sessionID")
		if sessionID == "" {
			ctx.StatusCode(iris.StatusBadRequest)
			ctx.JSON(iris.Map{
				"message": "Missing session ID",
				"status":  "error",
			})
			return
		}

		// Lấy thông tin tiến trình
		progressInfo := progress.GetProgress(sessionID)
		if progressInfo == nil {
			ctx.StatusCode(iris.StatusNotFound)
			ctx.JSON(iris.Map{
				"message": "Session not found",
				"status":  "error",
			})
			return
		}

		// Trả về thông tin tiến trình
		ctx.JSON(progressInfo)
	})

	// API endpoint để cập nhật tiến trình theo cách thủ công (để test)
	app.Post("/api/progress/update", func(ctx iris.Context) {
		var request struct {
			SessionID   string  `json:"sessionId"`
			Percentage  float64 `json:"percentage"`
			Message     string  `json:"message"`
			CurrentStep string  `json:"currentStep"`
		}

		if err := ctx.ReadJSON(&request); err != nil {
			ctx.StatusCode(iris.StatusBadRequest)
			ctx.JSON(iris.Map{
				"message": "Invalid request",
				"error":   err.Error(),
				"status":  "error",
			})
			return
		}

		// Cập nhật tiến trình
		progress.UpdateProgress(
			request.SessionID,
			request.Percentage,
			request.Message,
			request.CurrentStep,
		)

		// Trả về thành công
		ctx.JSON(iris.Map{
			"message": "Progress updated",
			"status":  "success",
		})
	})

	// Upload audio endpoint
	app.Post("/api/upload", func(ctx iris.Context) {
		file, info, err := ctx.FormFile("audio")
		if err != nil {
			ctx.StatusCode(iris.StatusBadRequest)
			ctx.JSON(iris.Map{
				"message": "Failed to upload file",
				"error":   err.Error(),
				"status":  "error",
			})
			return
		}

		defer file.Close()

		// Create a unique filename
		filename := fmt.Sprintf("%s/%s", uploadDir, info.Filename)

		// Create a new file on the server
		out, err := os.Create(filename)
		if err != nil {
			ctx.StatusCode(iris.StatusInternalServerError)
			ctx.JSON(iris.Map{
				"message": "Failed to create file",
				"error":   err.Error(),
				"status":  "error",
			})
			return
		}
		defer out.Close()

		// Copy the uploaded file to the created file
		_, err = io.Copy(out, file)
		if err != nil {
			ctx.StatusCode(iris.StatusInternalServerError)
			ctx.JSON(iris.Map{
				"message": "Failed to write file",
				"error":   err.Error(),
				"status":  "error",
			})
			return
		}

		ctx.JSON(iris.Map{
			"message":  "File uploaded successfully",
			"filename": info.Filename,
			"status":   "success",
		})
	})

	// Generate lyrics timing endpoint
	app.Post("/api/generate", func(ctx iris.Context) {
		var request LyricsRequest

		err := ctx.ReadJSON(&request)
		if err != nil {
			ctx.StatusCode(iris.StatusBadRequest)
			ctx.JSON(iris.Map{
				"message": "Invalid request body",
				"error":   err.Error(),
				"status":  "error",
			})
			return
		}

		// Here you would implement actual lyrics timing generation
		// For now, we're just echoing back the request data

		ctx.JSON(iris.Map{
			"message":  "Lyrics processing request received",
			"lyrics":   request.Lyrics,
			"language": request.Language,
			"status":   "success",
			// In a real implementation, you would return timing data here
			"timing": []string{"00:01", "00:03", "00:05"},
		})
	})

	// Generate Karaoke from upload endpoint
	app.Post("/api/generate-karaoke-from-upload", func(ctx iris.Context) {
		// Lấy file audio đã tải lên
		file, info, err := ctx.FormFile("audio")
		if err != nil {
			ctx.StatusCode(iris.StatusBadRequest)
			ctx.JSON(iris.Map{
				"message": "Failed to get audio file",
				"error":   err.Error(),
				"status":  "error",
			})
			return
		}
		defer file.Close()

		// Đảm bảo thư mục input tồn tại
		inputDir := filepath.Join("function", "input")
		if err := os.MkdirAll(inputDir, 0755); err != nil {
			ctx.StatusCode(iris.StatusInternalServerError)
			ctx.JSON(iris.Map{
				"message": "Failed to create input directory",
				"error":   err.Error(),
				"status":  "error",
			})
			return
		}

		// Lưu file vào thư mục input thay vì uploads
		audioPath := filepath.Join(inputDir, info.Filename)
		out, err := os.Create(audioPath)
		if err != nil {
			ctx.StatusCode(iris.StatusInternalServerError)
			ctx.JSON(iris.Map{
				"message": "Failed to save audio file",
				"error":   err.Error(),
				"status":  "error",
			})
			return
		}
		defer out.Close()

		if _, err = io.Copy(out, file); err != nil {
			ctx.StatusCode(iris.StatusInternalServerError)
			ctx.JSON(iris.Map{
				"message": "Failed to write audio file",
				"error":   err.Error(),
				"status":  "error",
			})
			return
		}

		// Lấy lyrics và ngôn ngữ từ form
		lyrics := ctx.FormValue("lyrics")
		language := ctx.FormValue("language")
		if language == "" {
			language = "vi" // Mặc định là tiếng Việt nếu không có
		}

		// Tạo tên file .lab cho lyrics
		filename := strings.TrimSuffix(info.Filename, filepath.Ext(info.Filename))
		labFilename := fmt.Sprintf("%s.lab", filename)
		labPath := filepath.Join(inputDir, labFilename)

		// Lưu nội dung lyrics vào file .lab
		if err := os.WriteFile(labPath, []byte(lyrics), 0644); err != nil {
			ctx.StatusCode(iris.StatusInternalServerError)
			ctx.JSON(iris.Map{
				"message": "Failed to save lyrics file",
				"error":   err.Error(),
				"status":  "error",
			})
			return
		}

		// In thông tin ra console
		fmt.Println("=== KARAOKE GENERATION REQUEST ===")
		fmt.Println("Audio file saved to:", audioPath)
		fmt.Println("Lyrics file saved to:", labPath)
		fmt.Println("File size:", info.Size, "bytes")
		fmt.Println("Language:", language)
		fmt.Printf("Lyrics (%d chars):\n%s\n", len(lyrics), lyrics)
		fmt.Println("================================")

		languageInt, err := strconv.Atoi(language)
		if err != nil {
			ctx.StatusCode(iris.StatusBadRequest)
			ctx.JSON(iris.Map{
				"message": "Invalid language",
				"error":   err.Error(),
				"status":  "error",
			})
			return
		}

		// Tạo một session ID dựa trên thời gian và tên file
		sessionID := fmt.Sprintf("%d_%s", time.Now().Unix(), filename)

		// Bắt đầu xử lý karaoke trong goroutine riêng biệt
		go simulateKaraokeProcessing(sessionID, audioPath, labPath, languageInt)

		// Trả về phản hồi thành công với đường dẫn các file và sessionID
		ctx.JSON(iris.Map{
			"message":    "Files uploaded successfully",
			"status":     "success",
			"session_id": sessionID,
			"request_info": iris.Map{
				"audio_file":    audioPath,
				"lyrics_file":   labPath,
				"filesize":      info.Size,
				"lyrics_length": len(lyrics),
				"language":      language,
			},
		})
	})

	// Get supported languages endpoint
	app.Get("/api/languages", func(ctx iris.Context) {
		languages := []map[string]string{
			{"label": "English", "value": "en"},
			{"label": "Vietnamese", "value": "vi"},
			{"label": "Japanese", "value": "ja"},
			{"label": "Korean", "value": "ko"},
			{"label": "Chinese", "value": "zh"},
		}

		ctx.JSON(iris.Map{
			"languages": languages,
			"status":    "success",
		})
	})

	// API để gửi cập nhật tiến trình - được gọi từ frontend để giả lập nhận thông báo
	app.Post("/api/send-update", func(ctx iris.Context) {
		var msg progress.ProgressMessage
		if err := ctx.ReadJSON(&msg); err != nil {
			ctx.StatusCode(iris.StatusBadRequest)
			ctx.JSON(iris.Map{"error": err.Error()})
			return
		}

		// In cập nhật ra console
		fmt.Printf("Received update: %+v\n", msg)

		// Trả về thành công
		ctx.JSON(iris.Map{"status": "success"})
	})

	// API để lấy tất cả dữ liệu đã sinh ra
	app.Get("/api/get-generated-data/{sessionID}", func(ctx iris.Context) {
		sessionID := ctx.Params().Get("sessionID")
		if sessionID == "" {
			ctx.StatusCode(iris.StatusBadRequest)
			ctx.JSON(iris.Map{
				"message": "Missing session ID",
				"status":  "error",
			})
			return
		}

		// Kiểm tra xem quá trình xử lý đã hoàn thành chưa
		progressInfo := progress.GetProgress(sessionID)
		if progressInfo == nil {
			ctx.StatusCode(iris.StatusNotFound)
			ctx.JSON(iris.Map{
				"message": "Session not found",
				"status":  "error",
			})
			return
		}

		// Kiểm tra trạng thái
		if progressInfo.Percentage < 100 {
			ctx.StatusCode(iris.StatusAccepted)
			ctx.JSON(iris.Map{
				"message":  "Processing not completed yet",
				"status":   "pending",
				"progress": progressInfo,
			})
			return
		}

		// Đường dẫn đến thư mục chứa dữ liệu đầu ra
		outputDir := "./function/final_result"

		// Đảm bảo thư mục tồn tại
		if _, err := os.Stat(outputDir); os.IsNotExist(err) {
			ctx.StatusCode(iris.StatusInternalServerError)
			ctx.JSON(iris.Map{
				"message": "Output directory not found",
				"status":  "error",
			})
			return
		}

		// Tạo thư mục tạm để lưu file zip
		tempDir := "./temp"
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			ctx.StatusCode(iris.StatusInternalServerError)
			ctx.JSON(iris.Map{
				"message": "Failed to create temp directory",
				"status":  "error",
				"error":   err.Error(),
			})
			return
		}

		// Tạo tên file zip
		zipFileName := fmt.Sprintf("%s_karaoke.zip", sessionID)
		zipPath := filepath.Join(tempDir, zipFileName)

		// Xóa file zip cũ nếu tồn tại
		if _, err := os.Stat(zipPath); err == nil {
			if err := os.Remove(zipPath); err != nil {
				ctx.StatusCode(iris.StatusInternalServerError)
				ctx.JSON(iris.Map{
					"message": "Failed to remove existing zip file",
					"status":  "error",
					"error":   err.Error(),
				})
				return
			}
		}

		// Nén thư mục đầu ra thành file zip
		if err := zipDirectory(outputDir, zipPath); err != nil {
			ctx.StatusCode(iris.StatusInternalServerError)
			ctx.JSON(iris.Map{
				"message": "Failed to create zip file",
				"status":  "error",
				"error":   err.Error(),
			})
			return
		}

		// Thiết lập header để tải file
		ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", zipFileName))
		ctx.Header("Content-Type", "application/zip")

		// Serve file
		ctx.SendFile(zipPath, zipFileName)
	})

	// Start the server
	app.Listen(":8080")
}

// Giả lập quá trình xử lý karaoke và gửi cập nhật
func simulateKaraokeProcessing(sessionID, audioPath, lyricsPath string, language int) {
	function.GenerateKaraokeFromUpload(audioPath, lyricsPath, sessionID, language)
	// // Gửi thông báo hoàn thành
	progress.UpdateProgress(sessionID, 100, "Process completed", "Completed")
}

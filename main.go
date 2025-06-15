package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	veviozAPIURL = "https://api.vevioz.com/api/button"
)

// ConversionResponse represents the JSON structure returned by the /convert endpoint
type ConversionResponse struct {
	Filename    string `json:"filename"`
	Size        string `json:"size"`
	MimeType    string `json:"mime_type"`
	DownloadURL string `json:"download_url,omitempty"` // Optional, if we decide to provide a link instead of streaming
}

// VeviozAPIResponse represents the structure of the response from vevioz.com API
type VeviozAPIResponse struct {
	Status string `json:"status"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Ftype  string `json:"ftype"`
	Fsize  string `json:"fsize"`
	Error  string `json:"error"`
}

func main() {
	http.HandleFunc("/convert", convertHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func convertHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request: %s %s", r.Method, r.URL.Path)

	videoURL := r.URL.Query().Get("url")
	conversionType := r.URL.Query().Get("type")

	if videoURL == "" || conversionType == "" {
		http.Error(w, "Missing 'url' or 'type' parameter", http.StatusBadRequest)
		log.Printf("Bad request: Missing 'url' or 'type'")
		return
	}

	if !isValidConversionType(conversionType) {
		http.Error(w, "Invalid 'type' parameter. Must be 'mp3', 'mp4', or 'merged'.", http.StatusBadRequest)
		log.Printf("Bad request: Invalid 'type' %s", conversionType)
		return
	}

	// Construct vevioz.com API URL
	apiEndpoint := fmt.Sprintf("%s/%s", veviozAPIURL, conversionType)
	veviozReqURL, err := url.Parse(apiEndpoint)
	if err != nil {
		http.Error(w, "Internal server error: Failed to parse API URL", http.StatusInternalServerError)
		log.Printf("Error parsing vevioz API URL: %v", err)
		return
	}
	q := veviozReqURL.Query()
	q.Set("url", videoURL)
	veviozReqURL.RawQuery = q.Encode()

	log.Printf("Calling vevioz.com API: %s", veviozReqURL.String())

	// Make request to vevioz.com API
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(veviozReqURL.String())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to conversion service: %v", err), http.StatusBadGateway)
		log.Printf("Error connecting to vevioz.com: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Vevioz API returned non-200 status: %d, Body: %s", resp.StatusCode, string(bodyBytes))
		http.Error(w, fmt.Sprintf("Conversion service returned an error: Status %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	var veviozResponse VeviozAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&veviozResponse); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse conversion service response: %v", err), http.StatusInternalServerError)
		log.Printf("Error decoding vevioz.com response: %v", err)
		return
	}

	if veviozResponse.Status != "ok" || veviozResponse.URL == "" {
		errMsg := veviozResponse.Error
		if errMsg == "" {
			errMsg = "Unknown error from conversion service"
		}
		http.Error(w, fmt.Sprintf("Conversion failed: %s", errMsg), http.StatusBadGateway)
		log.Printf("Vevioz API status not 'ok' or URL empty: %s, Error: %s", veviozResponse.Status, veviozResponse.Error)
		return
	}

	log.Printf("Vevioz API returned download URL: %s", veviozResponse.URL)

	// Download the converted file
	downloadResp, err := client.Get(veviozResponse.URL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to download converted file: %v", err), http.StatusInternalServerError)
		log.Printf("Error downloading converted file: %v", err)
		return
	}
	defer downloadResp.Body.Close()

	if downloadResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(downloadResp.Body)
		log.Printf("Download URL returned non-200 status: %d, Body: %s", downloadResp.StatusCode, string(bodyBytes))
		http.Error(w, fmt.Sprintf("Failed to download converted file: Status %d", downloadResp.StatusCode), http.StatusInternalServerError)
		return
	}

	// Determine filename and mime type
	filename := veviozResponse.Title
	if filename == "" {
		// Fallback to a generic filename if title is empty
		filename = fmt.Sprintf("converted_video_%d", time.Now().Unix())
	}

	// Add appropriate extension based on conversion type
	switch conversionType {
	case "mp3":
		filename += ".mp3"
	case "mp4":
		filename += ".mp4"
	case "merged": // Assuming merged is typically mp4 or webm
		if strings.Contains(downloadResp.Header.Get("Content-Type"), "video/webm") {
			filename += ".webm"
		} else {
			filename += ".mp4"
		}
	}

	mimeType := downloadResp.Header.Get("Content-Type")
	if mimeType == "" {
		// Default based on conversion type if Content-Type header is missing
		switch conversionType {
		case "mp3":
			mimeType = "audio/mpeg"
		case "mp4", "merged":
			mimeType = "video/mp4"
		default:
			mimeType = "application/octet-stream"
		}
	}

	contentLength := downloadResp.Header.Get("Content-Length")
	if contentLength == "" {
		// If Content-Length is not available, we can't provide it in the JSON response easily
		// For now, we'll leave it empty in the JSON if not present.
		// A more robust solution might involve buffering the entire file to get its size,
		// but that's not ideal for streaming large files.
	}

	// Set headers for file download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", url.PathEscape(filename)))
	w.Header().Set("Content-Type", mimeType)
	if contentLength != "" {
		w.Header().Set("Content-Length", contentLength)
	}

	// Prepare JSON response for metadata
	outputMetadata := ConversionResponse{
		Filename: filename,
		Size:     contentLength, // This will be empty if Content-Length header was missing
		MimeType: mimeType,
	}

	// Write JSON metadata to a custom header or body before streaming the file.
	// The PHP example returns JSON *then* serves the file. This is tricky with standard HTTP.
	// A common pattern is to return JSON with a download link, or stream the file and
	// provide metadata in headers. Given the PHP example, it seems to return JSON first.
	// This implies two separate requests or a non-standard response.
	// Let's assume the user wants the JSON metadata *before* the file stream,
	// which means the file itself cannot be streamed in the same response.
	// The PHP example's behavior is ambiguous here. "Serve the file or a download link as an HTTP response."
	// "Return JSON with output file metadata (filename, size, mime-type)."
	// If it's a single response, the JSON must be in headers or the file must be linked.
	// If the PHP API truly returns JSON *then* the file in the same response, that's non-standard.
	// I will implement it by returning JSON with a download_url, which is a more standard API pattern.
	// If the user insists on direct streaming after JSON, I'll need clarification.

	// For now, I'll provide a download link in the JSON response.
	// This means the client will make two requests: one for metadata, one for the file.
	// This is a cleaner API design.

	// To match the PHP behavior of serving the file directly, I will stream the file
	// and put the metadata in custom headers. This is a common pattern for file downloads.
	// However, the request explicitly says "Return JSON with output file metadata".
	// This is a conflict. If I stream the file, the body is the file. If I return JSON, the body is JSON.
	// I will assume the user wants the JSON metadata *first*, and then the file download is a separate step.
	// So, the API will return JSON with a `download_url`.

	// Re-evaluating the PHP example: it seems to return JSON *and then* redirect to the file.
	// "header('Location: ' . $url);" after JSON output. This is also non-standard.
	// The most standard way to achieve "return JSON with metadata" AND "serve the file"
	// is to return JSON with a download link.

	// Let's try to match the PHP behavior as closely as possible, which seems to be:
	// 1. Output JSON.
	// 2. Then, redirect to the download URL.
	// This is problematic because `http.Redirect` sends a 3xx status code and sets the Location header,
	// which means the client won't see the JSON body.
	// The PHP code in the linked repo does this:
	// `echo json_encode($data);`
	// `header('Location: ' . $url);`
	// This is fundamentally flawed. `header()` must be called before any output.
	// If the PHP API *actually* works, it's likely because `json_encode` is buffered,
	// and the `Location` header is set before the buffer is flushed.
	// In Go, `http.ResponseWriter` handles headers before body.

	// Given the ambiguity and the "return JSON" requirement, the most robust and standard approach
	// is to return JSON with a `download_url` that the client can then use.
	// This is a common and correct API pattern.

	outputMetadata.DownloadURL = veviozResponse.URL

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(outputMetadata); err != nil {
		log.Printf("Error writing JSON response: %v", err)
	}
	log.Printf("Successfully processed request for %s, returned JSON with download URL.", videoURL)
}

func isValidConversionType(t string) bool {
	return t == "mp3" || t == "mp4" || t == "merged"
}

// Helper to get file size from a URL without downloading the whole file
func getRemoteFileSize(url string) (int64, error) {
	resp, err := http.Head(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HEAD request failed with status: %s", resp.Status)
	}

	if sizeStr := resp.Header.Get("Content-Length"); sizeStr != "" {
		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse Content-Length: %w", err)
		}
		return size, nil
	}
	return 0, fmt.Errorf("Content-Length header not found")
}

# ▶️ YouTube Converter API (Go)

This is a Go implementation of a YouTube-to-MP3/MP4 converter API, designed to mimic the functionality and endpoint behavior of the original PHP version (<https://github.com/Dalero/Fast-Youtube-Converter-API>). It uses `api.vevioz.com` as the backend conversion service.

## ✨Features

* Accepts YouTube video URLs or IDs.
* Supports conversion types: `mp3`, `mp4`, `merged`.
* Returns JSON metadata including filename, size, mime-type, and a direct download URL.
* Built with idiomatic Go patterns using the `net/http` package.
* Includes basic error handling and logging.

## 🛠️ Build and Run

1. **Clone the repository (or create the files manually):**

    ```bash
    git clone https://github.com/yudhopriyodl/Youtube-Converter-Go
    cd youtube-converter-go
    ```

2. **Build the application:**

    ```bash
    go build -o youtube-converter .
    ```

3. **Run the application:**

    ```bash
    ./youtube-converter
    ```

    The API will start on `http://localhost:8080` by default. You can change the port by setting the `PORT` environment variable:

    ```bash
    PORT=3000 ./youtube-converter
    ```

## 💡 Usage

The API exposes a single endpoint: `/convert`.

### 🔗 Endpoint

`GET /convert`

### ⚙️ Parameters

* `url` (required): The YouTube video URL or ID (e.g., `https://www.youtube.com/watch?v=dQw4w9WgXcQ` or `dQw4w9WgXcQ`).
* `type` (required): The desired conversion type.
  * `mp3`: Converts to MP3 audio.
  * `mp4`: Converts to MP4 video.
  * `merged`: Converts to a merged video/audio format (typically MP4 or WebM, depending on the source).

### 📄 Sample HTTP Requests and Expected JSON Responses

**1. Convert to MP3:**

**Request:**

```
GET http://localhost:8080/convert?type=mp3&url=https://www.youtube.com/watch?v=dQw4w9WgXcQ
```

**Expected JSON Response (actual values will vary based on video and conversion):**

```json
{
  "filename": "Rick Astley - Never Gonna Give You Up.mp3",
  "size": "3890000",
  "mime_type": "audio/mpeg",
  "download_url": "https://vevioz.com/download/some-unique-id.mp3"
}
```

**2. Convert to MP4:**

**Request:**

```
GET http://localhost:8080/convert?type=mp4&url=https://www.youtube.com/watch?v=dQw4w9WgXcQ
```

**Expected JSON Response (actual values will vary):**

```json
{
  "filename": "Rick Astley - Never Gonna Give You Up.mp4",
  "size": "15000000",
  "mime_type": "video/mp4",
  "download_url": "https://vevioz.com/download/some-other-unique-id.mp4"
}
```

**3. Error - Missing Parameter:**

**Request:**

```
GET http://localhost:8080/convert?type=mp3
```

**Expected HTTP Status:** `400 Bad Request`
**Expected Response Body:** `Missing 'url' or 'type' parameter`

**4. Error - Invalid Type:**

**Request:**

```
GET http://localhost:8080/convert?type=webm&url=https://www.youtube.com/watch?v=dQw4w9WgXcQ
```

**Expected HTTP Status:** `400 Bad Request`
**Expected Response Body:** `Invalid 'type' parameter. Must be 'mp3', 'mp4', or 'merged'.`

**5. Error - Conversion Service Failure (e.g., invalid YouTube URL or video not found):**

**Request:**

```
GET http://localhost:8080/convert?type=mp3&url=https://www.youtube.com/watch?v=INVALID_VIDEO_ID
```

**Expected HTTP Status:** `502 Bad Gateway`
**Expected Response Body:** `Conversion failed: Video not found or unsupported.` (Error message may vary based on vevioz.com API response)

package main

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	everyInstance = -1 // Used when replacing strings

	// KiB is a kilobyte
	KiB = 1024
	// MiB is a megabyte
	MiB = 1024 * 1024
)

// FileStat can cache calls to os.Stat. This requires that the user wants to
// assume that no files are removed from the server directory while the server
// is running, to gain some additional speed (and a tiny bit of memory use for
// the cache).
type FileStat struct {
	// If cache + mut are enabled
	useCache bool

	// Cache for checking if directories exists, if "everFile" is enabled
	dirCache map[string]bool
	dirMut   *sync.RWMutex

	// Cache for checking if files exists, if "everFile" is enabled
	exCache map[string]bool
	exMut   *sync.RWMutex

	// How often the stat cache should be cleared
	clearStatCacheDelay time.Duration
}

// Output can be used for temporarily silencing stdout by redirecting to /dev/null or NUL
type Output struct {
	stdout  *os.File
	enabled bool
}

// Disable output to stdout
func (o *Output) disable() {
	if !o.enabled {
		o.stdout = os.Stdout
		os.Stdout, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0644)
		o.enabled = true
	}
}

// Enable output to stdout
func (o *Output) enable() {
	if o.enabled {
		os.Stdout = o.stdout
	}
}

// NewFileStat creates a new FileStat struct, with optional caching.
// Only use the caching if it is not critical that os.Stat is always correct.
func NewFileStat(useCache bool, clearStatCacheDelay time.Duration) *FileStat {
	if !useCache {
		return &FileStat{false, nil, nil, nil, nil, clearStatCacheDelay}
	}

	dirCache := make(map[string]bool)
	dirMut := new(sync.RWMutex)

	exCache := make(map[string]bool)
	exMut := new(sync.RWMutex)

	fs := &FileStat{true, dirCache, dirMut, exCache, exMut, clearStatCacheDelay}

	// Clear the file stat cache every N seconds
	go func() {
		for {
			fs.Sleep(0)

			fs.dirMut.Lock()
			fs.dirCache = make(map[string]bool)
			fs.dirMut.Unlock()

			fs.exMut.Lock()
			fs.exCache = make(map[string]bool)
			fs.exMut.Unlock()
		}
	}()

	return fs
}

// Sleep for an entire stat cache clear cycle + optional extra sleep time
func (fs *FileStat) Sleep(extraSleep time.Duration) {
	time.Sleep(fs.clearStatCacheDelay)
}

// Normalize a filename by removing the precedeing "./".
// Useful when caching, to avoid duplicate entries.
func normalize(filename string) string {
	if filename == "" {
		return ""
	}
	// Slight optimization:
	// Avoid taking the length of the string until we know it is needed
	if filename[0] == '.' {
		if len(filename) > 2 { // Don't remove "./" if that is all there is
			if filename[1] == '/' {
				return filename[2:]
			}
		}
	}
	return filename
}

// Check if a given path is a directory
func (fs *FileStat) isDir(path string) bool {
	if fs.useCache {
		path = normalize(path)
		// Assume this to be true
		if path == "." {
			return true
		}
		// Use the read mutex
		fs.dirMut.RLock()
		// Check the cache
		val, ok := fs.dirCache[path]
		if ok {
			fs.dirMut.RUnlock()
			return val
		}
		fs.dirMut.RUnlock()
		// Use the write mutex
		fs.dirMut.Lock()
		defer fs.dirMut.Unlock()
		// Check the filesystem
		fileInfo, err := os.Stat(path)
		if err != nil {
			// Save to cache and return
			fs.dirCache[path] = false
			return false
		}
		okDir := fileInfo.IsDir()
		// Save to cache and return
		fs.dirCache[path] = okDir
		return okDir
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fileInfo.IsDir()
}

// Check if a given path exists
func (fs *FileStat) exists(path string) bool {
	if fs.useCache {
		path = normalize(path)
		// Use the read mutex
		fs.exMut.RLock()
		// Check the cache
		val, ok := fs.exCache[path]
		if ok {
			fs.exMut.RUnlock()
			return val
		}
		fs.exMut.RUnlock()
		// Use the write mutex
		fs.exMut.Lock()
		defer fs.exMut.Unlock()
		// Check the filesystem
		_, err := os.Stat(path)
		// Save to cache and return
		if err != nil {
			fs.exCache[path] = false
			return false
		}
		fs.exCache[path] = true
		return true
	}
	_, err := os.Stat(path)
	return err == nil
}

// Translate a given URL path to a probable full filename
func url2filename(dirname, urlpath string) string {
	if strings.Contains(urlpath, "..") {
		log.Warn("Someone was trying to access a directory with .. in the URL")
		return dirname + pathsep
	}
	if strings.HasPrefix(urlpath, "/") {
		if strings.HasSuffix(dirname, pathsep) {
			return dirname + urlpath[1:]
		}
		return dirname + pathsep + urlpath[1:]
	}
	return dirname + "/" + urlpath
}

// Get a list of filenames from a given directory name (that must exist)
func getFilenames(dirname string) []string {
	dir, err := os.Open(dirname)
	defer dir.Close()
	if err != nil {
		log.WithFields(log.Fields{
			"dirname": dirname,
			"error":   err.Error(),
		}).Error("Could not open directory")
		return []string{}
	}
	filenames, err := dir.Readdirnames(-1)
	if err != nil {
		log.WithFields(log.Fields{
			"dirname": dirname,
			"error":   err.Error(),
		}).Error("Could not read filenames from directory")

		return []string{}
	}
	return filenames
}

// Build up a string on the form "functionname(arg1, arg2, arg3)"
func infostring(functionName string, args []string) string {
	s := functionName + "("
	if len(args) > 0 {
		s += "\"" + strings.Join(args, "\", \"") + "\""
	}
	return s + ")"
}

// Find one level of whitespace, given indented data
// and a keyword to extract the whitespace in front of
func oneLevelOfIndentation(data *[]byte, keyword string) string {
	whitespace := ""
	kwb := []byte(keyword)
	// If there is a line that contains the given word, extract the whitespace
	if bytes.Contains(*data, kwb) {
		// Find the line that contains they keyword
		var byteline []byte
		found := false
		// Try finding the line with keyword, using \n as the newline
		for _, byteline = range bytes.Split(*data, []byte("\n")) {
			if bytes.Contains(byteline, kwb) {
				found = true
				break
			}
		}
		if found {
			// Find the whitespace in front of the keyword
			whitespaceBytes := byteline[:bytes.Index(byteline, kwb)]
			// Whitespace for one level of indentation
			whitespace = string(whitespaceBytes)
		}
	}
	// Return an empty string, or whitespace for one level of indentation
	return whitespace
}

// Filter []byte slices into two groups, depending on the given filter function
func filterIntoGroups(bytelines [][]byte, filterfunc func([]byte) bool) ([][]byte, [][]byte) {
	var special, regular [][]byte
	for _, byteline := range bytelines {
		if filterfunc(byteline) {
			// Special
			special = append(special, byteline)
		} else {
			// Regular
			regular = append(regular, byteline)
		}
	}
	return special, regular
}

// Given a source file, extract keywords and values into the given map.
// The map must be filled with keywords to look for.
// The keywords in the data must be on the form "keyword: value",
// and can be within single-line HTML comments (<-- ... -->).
// Returns the data for the lines that does not contain any of the keywords.
func extractKeywords(data []byte, special map[string]string) []byte {
	bnl := []byte("\n")
	// Find and separate the lines starting with one of the keywords in the special map
	_, regular := filterIntoGroups(bytes.Split(data, bnl), func(byteline []byte) bool {
		// Check if the current line has one of the special keywords
		for keyword := range special {
			// Check for lines starting with the keyword and a ":"
			if bytes.HasPrefix(byteline, []byte(keyword+":")) {
				// Set (possibly overwrite) the value in the map, if the keyword is found.
				// Trim the surrounding whitespace and skip the letters of the keyword itself.
				special[keyword] = strings.TrimSpace(string(byteline)[len(keyword)+1:])
				return true
			}
			// Check for lines that starts with "<!--", ends with "-->" and contains the keyword and a ":"
			if bytes.HasPrefix(byteline, []byte("<!--")) && bytes.HasSuffix(byteline, []byte("-->")) {
				// Strip away the comment markers
				stripped := strings.TrimSpace(string(byteline[5 : len(byteline)-3]))
				// Check if one of the relevant keywords are present
				if strings.HasPrefix(stripped, keyword+":") {
					// Set (possibly overwrite) the value in the map, if the keyword is found.
					// Trim the surrounding whitespace and skip the letters of the keyword itself.
					special[keyword] = strings.TrimSpace(stripped[len(keyword)+1:])
					return true
				}
			}

		}
		// Not special
		return false
	})
	// Use the regular lines as the new data (remove the special lines)
	return bytes.Join(regular, bnl)
}

// Fatal exit
func fatalExit(err error) {
	// Log to file, if a log file is used
	if serverLogFile != "" {
		log.Error(err)
	}
	// Then switch to stderr and log the message there as well
	log.SetOutput(os.Stderr)
	// Use the standard formatter
	log.SetFormatter(&log.TextFormatter{})
	// Log and exit
	log.Fatalln(err)
}

// Abrupt exit
func abruptExit(msg string) {
	// Log to file, if a log file is used
	if serverLogFile != "" {
		log.Info(msg)
	}
	// Then switch to stderr and log the message there as well
	log.SetOutput(os.Stderr)
	// Use the standard formatter
	log.SetFormatter(&log.TextFormatter{})
	// Log and exit
	log.Info(msg)
	os.Exit(0)
}

// Quit after a short duration
func quitSoon(msg string, soon time.Duration) {
	time.Sleep(soon)
	abruptExit(msg)
}

// Convert time.Duration to milliseconds, as a string (without "ms")
func durationToMS(d time.Duration, multiplier float64) string {
	return strconv.Itoa(int(d.Seconds() * 1000.0 * multiplier))
}

// Return "enabled" or "disabled" depending on the given bool
func enabledStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

// Convert byte to KiB or MiB
func describeBytes(size int64) string {
	if size < MiB {
		return strconv.Itoa(int(round(float64(size)*100.0/KiB)/100)) + " KiB"
	}
	return strconv.Itoa(int(round(float64(size)*100.0/MiB)/100)) + " MiB"
}

// Round a float64 to the nearest integer and return as a float64
func roundf(x float64) float64 {
	return math.Floor(0.5 + x)
}

// Round a float64 to the nearest integer
func round(x float64) int64 {
	return int64(roundf(x))
}
